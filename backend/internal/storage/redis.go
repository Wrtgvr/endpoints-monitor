package storage

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/wrtgvr/websites-monitor/internal/config"
	"github.com/wrtgvr/websites-monitor/internal/domain"
	errs "github.com/wrtgvr/websites-monitor/internal/errors"
)

type RedisStorage struct {
	client       *redis.Client
	maxEndpoints int64
}

func NewRedisStorage(cfg *config.RedisConfig) *RedisStorage {
	return &RedisStorage{
		client: redis.NewClient(&redis.Options{
			Addr:     cfg.Addr,
			Password: cfg.Password,
			DB:       cfg.DB,
		}),
		maxEndpoints: cfg.MaxEndpoints,
	}
}

func (s *RedisStorage) Close() error {
	return s.client.Close()
}

func (s *RedisStorage) AddEndpoint(ctx context.Context, ep *domain.EndpointInfo) *errs.AppError {
	//* check if theres maximum amount of endpoints
	count, err := s.client.ZCard(ctx, "endpoints:zset").Result()
	if err != nil {
		return errs.NewInternalError(fmt.Errorf("failed to get amount of endpoints: err=%w", err))
	}
	if count >= s.maxEndpoints {
		return errs.NewAppError(err, fmt.Sprintf("too many endpoint, amount=%d", count), errs.TypeConflict, http.StatusConflict)
	}

	//* generate uuid
	id := uuid.New().String()

	//* prepare pipeline
	pipe := s.client.Pipeline()

	pipe.ZAdd(ctx, "endpoints:zset", redis.Z{
		Score:  float64(time.Now().Unix()),
		Member: id,
	})

	pipe.HSet(ctx, fmt.Sprintf("endpoint:%s:info", id),
		"url", ep.URL,
		"name", ep.Name,
	)

	pipe.HSet(ctx, fmt.Sprintf("endpoint:%s:status", id),
		"status", "unknown",
		"last_checked", "0",
		"response_time", "0",
	)

	//* add endpoint
	_, err = pipe.Exec(ctx)
	if err != nil {
		return errs.NewInternalError(fmt.Errorf("pipe execution failed err=%w", err))
	}

	return nil
}

func (s *RedisStorage) DeleteEndpoint(ctx context.Context, id string) *errs.AppError {
	//* prepare pipelineeeeee
	pipe := s.client.Pipeline()

	pipe.ZRem(ctx, "endpoints:zset", id)
	pipe.Del(ctx, fmt.Sprintf("endpoint:%s:info", id))
	pipe.Del(ctx, fmt.Sprintf("endpoint:%s:status", id))

	//* execute
	_, err := pipe.Exec(ctx)
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return errs.NewAppError(err, fmt.Sprintf("endpoint with not found: id=%s", id), errs.TypeNotFound, http.StatusNotFound)
		}
		return errs.NewInternalError(fmt.Errorf("id=%s, err=%w", id, err))
	}

	return nil
}

func (s *RedisStorage) ChangeEndpointInfo(ctx context.Context, info *domain.EndpointInfo) *errs.AppError {
	//* get current url and name
	res, err := s.client.HGetAll(ctx, fmt.Sprintf("endpoint:%s:info", info.ID)).Result()
	if err != nil {
		return errs.NewInternalError(fmt.Errorf("failed to get endpoint info: id=%s, err=%w", info.ID, err))
	}

	//* if url or name from given info struct is nil value ("") then use enpodint current url/name
	url := info.URL
	if url == "" {
		url = res["url"]
	}
	name := info.Name
	if name == "" {
		name = res["name"]
	}

	//* update endpoint
	err = s.client.HSet(ctx, fmt.Sprintf("endpoint:%s:info", info.ID),
		"url", url,
		"name", name,
	).Err()
	if err != nil {
		return errs.NewInternalError(fmt.Errorf("failed to update endpoint info: req_id=%s, req_name=%s, req_url=%s, err=%w", info.ID, info.Name, info.URL, err))
	}

	return nil
}

func (s *RedisStorage) UpdateEndpointStatus(ctx context.Context, endpointStatus *domain.EndpointStatus) *errs.AppError {
	err := s.client.HSet(ctx, fmt.Sprintf("endpoint:%s:status", endpointStatus.ID),
		"status", endpointStatus.Status,
		"last_checked", endpointStatus.LastChecked,
		"response_time", endpointStatus.ResponseTime,
	).Err()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return errs.NewAppError(err, fmt.Sprintf("endpoint with not found id=%s", endpointStatus.ID), errs.TypeNotFound, http.StatusNotFound)
		}
		return errs.NewInternalError(fmt.Errorf("id=%s, err=%w", endpointStatus.ID, err))
	}
	return nil
}

func (s *RedisStorage) GetEndpoints(ctx context.Context, limit, offset int64) ([]*domain.Endpoint, *errs.AppError) {
	//* get ids of endpoints
	ids, err := s.client.ZRange(ctx, "endpoints:zset", offset, limit+offset-1).Result()
	if err != nil {
		return nil, errs.NewInternalError(fmt.Errorf("limit=%d, offset=%d, err=%w", limit, offset, err))
	}

	//* prepare pipeline for info and status and execute cmds
	pipe := s.client.Pipeline()

	infoCmds := make(map[string]*redis.MapStringStringCmd)
	statusCmds := make(map[string]*redis.MapStringStringCmd)
	for _, id := range ids {
		infoCmds[id] = pipe.HGetAll(ctx, fmt.Sprintf("endpoint:%s:info", id))
		statusCmds[id] = pipe.HGetAll(ctx, fmt.Sprintf("endpoint:%s:status", id))
	}

	if _, err := pipe.Exec(ctx); err != nil {
		return nil, errs.NewInternalError(fmt.Errorf("pipe execution failed: %w", err))
	}

	//* get result
	endpoints := make([]*domain.Endpoint, 0, len(ids))
	failedEndpoints := make([]string, 0, len(ids))
	for _, id := range ids {
		info, err := infoCmds[id].Result()
		if err != nil {
			log.Printf("WARN: failed to get endpoint, id=%s, err=%v\n", id, err)
			failedEndpoints = append(failedEndpoints, id)
			continue
		}

		// check for required field
		if info["url"] == "" {
			log.Printf("WARN: endpoint does not have required URL field, id=%s, err=%v\n", id, err)
			failedEndpoints = append(failedEndpoints, id)
			continue
		}

		status, err := statusCmds[id].Result()
		if err != nil {
			return nil, errs.NewInternalError(fmt.Errorf("failed to get endpoint status cmd result, err=%w", err))
		}

		endpoints = append(endpoints, &domain.Endpoint{
			ID:           id,
			Name:         info["name"],
			URL:          info["url"],
			Status:       status["status"],
			LastChecked:  status["last_checked"],
			ResponseTime: status["response_time"],
		})
	}

	if len(failedEndpoints) > 0 {
		log.Printf("WARN: failed to load %d endpoints\n", len(failedEndpoints))
	}

	return endpoints, nil
}

func (s *RedisStorage) GetEndpointsForMonitoring(ctx context.Context) ([]*domain.EndpointInfo, *errs.AppError) {
	//* get ids of endpoints
	ids, err := s.client.ZRange(ctx, "endpoints:zset", 0, -1).Result()
	if err != nil {
		return nil, errs.NewInternalError(fmt.Errorf("failed to get endpoints ids: err=%w", err))
	}

	//* prepare pipeline
	pipe := s.client.Pipeline()

	cmds := make(map[string]*redis.MapStringStringCmd)
	for _, id := range ids {
		cmds[id] = pipe.HGetAll(ctx, fmt.Sprintf("endpoint:%s:info", id))
	}

	if _, err := pipe.Exec(ctx); err != nil {
		return nil, errs.NewInternalError(fmt.Errorf("failed to execute pipeline: %w", err))
	}

	//* get result
	endpoints := make([]*domain.EndpointInfo, 0, len(ids))
	failedEndpoints := make([]string, 0, len(ids))
	for _, id := range ids {
		info, err := cmds[id].Result()
		if err != nil {
			log.Printf("WARN: failed to get endpoint, id=%s, err=%v\n", id, err)
			failedEndpoints = append(failedEndpoints, id)
			continue
		}

		// check for required field
		if info["url"] == "" {
			log.Printf("WARN: endpoint does not have required URL field, id=%s, err=%v\n", id, err)
			failedEndpoints = append(failedEndpoints, id)
			continue
		}

		endpoints = append(endpoints, &domain.EndpointInfo{
			ID:   id,
			Name: info["name"],
			URL:  info["url"],
		})
	}

	if len(failedEndpoints) > 0 {
		log.Printf("WARN: failed to load %d endpoints for monitoring\n", len(failedEndpoints))
	}

	return endpoints, nil
}
