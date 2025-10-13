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

const (
	KeyTypeReadOnly = "read_only"
	KeyTypeAdmin    = "admin"
)

type RedisStorage struct {
	client          *redis.Client
	maxEndpoints    int64
	maxReadOnlyKeys int64
}

func NewRedisStorage(cfg *config.RedisConfig) *RedisStorage {
	return &RedisStorage{
		client: redis.NewClient(&redis.Options{
			Addr:     cfg.Addr,
			Password: cfg.Password,
			DB:       cfg.DB,
		}),
		maxEndpoints:    cfg.MaxEndpoints,
		maxReadOnlyKeys: cfg.MaxReadOnlyKeys,
	}
}

func (s *RedisStorage) Close() error {
	return s.client.Close()
}

// * Projects

func (s *RedisStorage) CreateProject(ctx context.Context, apiKey, name string) *errs.AppError {
	//* generate project id
	id := uuid.New().String()

	//* check if admin api key is already exists
	exists, err := s.client.Exists(ctx, s.key_ProjectByAdminAPIKey(apiKey)).Result()
	if err != nil {
		return errs.NewInternalError(fmt.Errorf("failed to check api key existence: %w", err))
	}
	if exists > 0 {
		return errs.NewConflict(nil, "api key already exists")
	}

	//* prepare pipeline
	pipe := s.client.TxPipeline()

	// project info
	pipe.HSet(ctx, s.key_ProjectInfo(id),
		"name", name,
	)

	// for quick search by api key
	pipe.HSet(ctx, s.key_ProjectByAdminAPIKey(apiKey), "id", id)

	//* execute pipeline
	_, err = pipe.Exec(ctx)
	if err != nil {
		return errs.NewInternalError(fmt.Errorf("failed to create a project, err=%w", err))
	}

	return nil
}

func (s *RedisStorage) GetProjectIDByAPIKey(ctx context.Context, apiKey string) (projectId string, appErr *errs.AppError) {
	id, err := s.client.HGet(ctx, s.key_ProjectByAdminAPIKey(apiKey), "id").Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return "", errs.NewNotFound(
				err,
				"project with given admin api key not found")
		}
		return "", errs.NewInternalError(
			fmt.Errorf("failed to get get project id by admin api key, err=%w", err))
	}

	return id, nil
}

func (s *RedisStorage) GetProjectInfo(ctx context.Context, projectId string) (*domain.Project, *errs.AppError) {
	info, err := s.client.HGetAll(ctx, s.key_ProjectInfo(projectId)).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, errs.NewNotFound(
				err,
				fmt.Sprintf("project with given id not found, project_id=%s", projectId),
			)
		}
		return nil, errs.NewInternalError(
			fmt.Errorf("failed to get project info, project_id=%s, err=%w", projectId, err))
	}

	return &domain.Project{
		ID:   projectId,
		Name: info["name"],
	}, nil
}

func (s *RedisStorage) ChangeProjectName(ctx context.Context, projectId, name string) *errs.AppError {
	//* check if project exists
	n, err := s.client.Exists(ctx, s.key_ProjectInfo(projectId)).Result()
	if err != nil {
		return errs.NewInternalError(err)
	}

	if n <= 0 {
		return errs.NewNotFound(nil, fmt.Sprintf("project with given id not found, project_id=%s", projectId))
	}

	//* update project info
	if err := s.client.HSet(ctx, s.key_ProjectInfo(projectId), "name", name).Err(); err != nil {
		return errs.NewInternalError(
			fmt.Errorf("failed to update project name, project_id=%s, err=%w", projectId, err))
	}
	return nil
}

// * api key

func (s *RedisStorage) UpdateProjectAdminAPIKey(ctx context.Context, oldApiKey, newApiKey, projectId string) *errs.AppError {
	//* prepare pipeine
	pipe := s.client.TxPipeline()

	// delete old, set new
	pipe.Del(ctx, s.key_ProjectByAdminAPIKey(oldApiKey))
	pipe.Del(ctx, s.key_ProjectKeyInfo(projectId, oldApiKey))
	pipe.ZRem(ctx, s.key_ProjectAPIKeys(projectId), oldApiKey)
	pipe.HSet(ctx, s.key_ProjectByAdminAPIKey(newApiKey), "id", projectId)
	pipe.HSet(ctx, s.key_ProjectKeyInfo(projectId, newApiKey),
		"type", KeyTypeAdmin,
		"createdAt", time.Now().Format(time.RFC3339))
	pipe.ZAdd(ctx, s.key_ProjectAPIKeys(projectId), redis.Z{
		Score:  float64(time.Now().Unix()),
		Member: newApiKey,
	})

	//* exec
	_, err := pipe.Exec(ctx)
	if err != nil {
		if appErr, ok := err.(*errs.AppError); ok {
			return appErr
		}
		return errs.NewInternalError(
			fmt.Errorf("failed to update project admin api key, id=%s, err=%s", projectId, err))
	}
	return nil
}

func (s *RedisStorage) AddReadonlyAPIKey(ctx context.Context, projectId, key string) *errs.AppError {
	//* check amount of keys
	n, err := s.client.ZCard(ctx, s.key_ProjectAPIKeys(projectId)).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return errs.NewNotFound(err,
				fmt.Sprintf("project read-only api keys value not found: project_id=%s", projectId))
		}
		return errs.NewInternalError(
			fmt.Errorf("failed to get amount of zset members: project_id=%s, err=%w",
				projectId, err))
	}
	if n >= s.maxReadOnlyKeys {
		return errs.NewConflict(nil, fmt.Sprintf("A project cannot have more than %d read-only api keys", s.maxReadOnlyKeys))
	}

	//* prepare pipeline
	pipe := s.client.TxPipeline()
	pipe.ZAdd(ctx, s.key_ProjectAPIKeys(projectId), redis.Z{
		Score:  float64(time.Now().Unix()),
		Member: key,
	})
	pipe.HSet(ctx, s.key_ProjectKeyInfo(projectId, key),
		"type", KeyTypeReadOnly,
		"createdAt", time.Now().Format(time.RFC3339))

	//* exec
	_, err = pipe.Exec(ctx)
	if err != nil {
		return errs.NewInternalError(
			fmt.Errorf("failed to add read-only api key: project_id=%s, err=%w", projectId, err))
	}

	return nil
}

func (s *RedisStorage) RemoveReadonlyAPIKey(ctx context.Context, projectId, key string) *errs.AppError {
	//* prepare pipeline
	pipe := s.client.TxPipeline()

	pipe.ZRem(ctx, s.key_ProjectAPIKeys(projectId), key)
	pipe.HDel(ctx, s.key_ProjectKeyInfo(projectId, key))

	//* exec
	_, err := pipe.Exec(ctx)
	if err != nil {
		return errs.NewInternalError(
			fmt.Errorf("failed to remove read-only key, project_id=%s, err=%w",
				projectId, err))
	}
	return nil
}

func (s *RedisStorage) GetReadonlyKeys(ctx context.Context, projectId string) ([]string, *errs.AppError) {
	keys, err := s.client.ZRange(ctx, s.key_ProjectAPIKeys(projectId), 0, -1).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, errs.NewNotFound(err,
				fmt.Sprintf("project read-only api keys value not found: project_id=%s", projectId))
		}
		return nil, errs.NewInternalError(
			fmt.Errorf("failed to get read-only api keys ids: err=%w", err))
	}

	readOnlyKeys := make([]string, 0)
	failedKeys := 0
	for _, key := range keys {
		keyType, err := s.client.HGet(ctx, s.key_ProjectKeyInfo(projectId, key), "type").Result()
		if err != nil {
			log.Printf("WARN: failed to get key type, project_id=%s, err=%v\n", projectId, err)
			failedKeys++
			continue
		}
		if keyType != KeyTypeReadOnly && keyType != KeyTypeAdmin {
			log.Printf("WARN: key has unknown type: key_type=%s\n", keyType)
			failedKeys++
			continue
		}
		readOnlyKeys = append(readOnlyKeys, key)
	}

	if failedKeys > 0 {
		log.Printf("WARN: failed to load %d endpoints\n", failedKeys)
	}

	return readOnlyKeys, nil
}

//* Endpoints

func (s *RedisStorage) GetEndpoints(ctx context.Context, projectId, endpointId string) ([]*domain.Endpoint, *errs.AppError) {
	//* get ids of endpoints
	ids, err := s.client.ZRange(ctx, s.key_ProjectEndpoints(projectId), 0, -1).Result()
	if err != nil {
		return nil, errs.NewInternalError(err)
	}

	//* prepare pipeline for info and status and execute cmds
	pipe := s.client.Pipeline()

	infoCmds := make(map[string]*redis.MapStringStringCmd)
	statusCmds := make(map[string]*redis.MapStringStringCmd)
	for _, endpointID := range ids {
		infoCmds[endpointID] = pipe.HGetAll(ctx, s.key_EndpointInfo(projectId, endpointID))
		statusCmds[endpointID] = pipe.HGetAll(ctx, s.key_EndpointStatus(projectId, endpointID))
	}

	if _, err := pipe.Exec(ctx); err != nil {
		return nil, errs.NewInternalError(fmt.Errorf("pipe execution failed: %w", err))
	}

	//* get result
	endpoints := make([]*domain.Endpoint, 0, len(ids))
	failedEndpoints := 0
	for _, id := range ids {
		info, err := infoCmds[id].Result()
		if err != nil {
			log.Printf("WARN: failed to get endpoint, id=%s, err=%v\n", id, err)
			failedEndpoints++
			continue
		}

		// check for required field
		if info["url"] == "" {
			log.Printf("WARN: endpoint does not have required URL field, id=%s, err=%v\n", id, err)
			failedEndpoints++
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

	if failedEndpoints > 0 {
		log.Printf("WARN: failed to load %d endpoints\n", failedEndpoints)
	}

	return endpoints, nil
}

func (s *RedisStorage) GetEndpointsForMonitoring(ctx context.Context, projectId string) ([]*domain.EndpointInfo, *errs.AppError) {
	//* get ids of endpoints
	ids, err := s.client.ZRange(ctx, s.key_ProjectEndpoints(projectId), 0, -1).Result()
	if err != nil {
		return nil, errs.NewInternalError(err)
	}

	//* prepare pipeline for info and status and execute cmds
	pipe := s.client.Pipeline()

	infoCmds := make(map[string]*redis.MapStringStringCmd)
	for _, endpointID := range ids {
		infoCmds[endpointID] = pipe.HGetAll(ctx, s.key_EndpointInfo(projectId, endpointID))
	}

	if _, err := pipe.Exec(ctx); err != nil {
		return nil, errs.NewInternalError(fmt.Errorf("pipe execution failed: %w", err))
	}

	//* get result
	endpoints := make([]*domain.EndpointInfo, 0, len(ids))
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
		endpoints = append(endpoints, &domain.EndpointInfo{
			ID:   id,
			Name: info["name"],
			URL:  info["url"],
		})
	}

	if len(failedEndpoints) > 0 {
		log.Printf("WARN: failed to load %d endpoints\n", len(failedEndpoints))
	}

	return endpoints, nil
}

func (s *RedisStorage) CreateEndpoint(ctx context.Context, projectId string, endpointInfo *domain.EndpointInfo) *errs.AppError {
	//* check if project exists
	n, err := s.client.Exists(ctx, s.key_ProjectInfo(projectId)).Result()
	if err != nil {
		return errs.NewInternalError(
			fmt.Errorf("failed to get project: project_id=%s, err=%w", projectId, err))
	}

	if n == 0 {
		return errs.NewNotFound(err,
			fmt.Sprintf("project not found: id=%s", projectId))
	}

	//* check amount of keys
	n, err = s.client.ZCard(ctx, s.key_ProjectEndpoints(projectId)).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return errs.NewNotFound(err,
				fmt.Sprintf("project endpoints value not found: project_id=%s", projectId))
		}
		return errs.NewInternalError(
			fmt.Errorf("failed to get amount of zset members: project_id=%s, err=%w",
				projectId, err))
	}
	if n >= s.maxEndpoints {
		return errs.NewConflict(nil, fmt.Sprintf("A project cannot have more than %d endpoints", s.maxReadOnlyKeys))
	}

	//* generate endpoint id
	epId := uuid.New().String()

	//* prepare pipeline
	tx := s.client.TxPipeline()

	s.client.ZAdd(ctx, s.key_ProjectEndpoints(projectId), redis.Z{
		Score:  float64(time.Now().Unix()),
		Member: epId,
	})
	s.client.HSet(ctx, s.key_EndpointInfo(projectId, epId),
		"name", endpointInfo.Name,
		"url", endpointInfo.URL)
	s.client.HSet(ctx, s.key_EndpointStatus(projectId, epId),
		"status", "Unknown",
		"last_checked", "Never",
		"response_time", "0")

	_, err = tx.Exec(ctx)
	if err != nil {
		errs.NewInternalError(fmt.Errorf("failed to create new endpoint: project_id=%s, err=%w", projectId, err))
	}

	return nil
}

func (s *RedisStorage) UpdateEndpointInfo(ctx context.Context, projectId string, endpointInfo *domain.EndpointInfo) *errs.AppError {
	//* get current url and name
	res, err := s.client.HGetAll(ctx, s.key_EndpointInfo(projectId, endpointInfo.ID)).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return errs.NewNotFound(err,
				fmt.Sprintf("endpoint not found: id=%s", endpointInfo.ID))
		}
		return errs.NewInternalError(
			fmt.Errorf("failed to get endpoint info: id=%s, err=%w", endpointInfo.ID, err))
	}

	//* if url or name from given info struct is nil value ("") then use enpodint current url/name
	url := endpointInfo.URL
	if url == "" {
		url = res["url"]
	}
	name := endpointInfo.Name
	if name == "" {
		name = res["name"]
	}

	//* update endpoint
	err = s.client.HSet(ctx, s.key_EndpointInfo(projectId, endpointInfo.ID),
		"url", url,
		"name", name,
	).Err()
	if err != nil {
		return errs.NewInternalError(
			fmt.Errorf("failed to update endpoint info: req_id=%s, req_name=%s, req_url=%s, err=%w", endpointInfo.ID, endpointInfo.Name, endpointInfo.URL, err))
	}

	return nil
}

func (s *RedisStorage) UpdateEndpointStatus(ctx context.Context, projectId string, endpointStatus *domain.EndpointStatus) *errs.AppError {
	//* check if endpoint exists
	n, err := s.client.Exists(ctx, s.key_EndpointInfo(projectId, endpointStatus.ID)).Result()
	if err != nil {
		return errs.NewInternalError(
			fmt.Errorf("failed to get endpoint info: id=%s, err=%w", endpointStatus.ID, err))
	}

	if n == 0 {
		return errs.NewNotFound(err,
			fmt.Sprintf("endpoint not found: id=%s", endpointStatus.ID))
	}

	//* update endpoint status
	err = s.client.HSet(ctx, s.key_EndpointInfo(projectId, endpointStatus.ID),
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

func (s *RedisStorage) DeleteEndpoint(ctx context.Context, projectId string, endpointId string) *errs.AppError {
	//* check if project exists
	n, err := s.client.Exists(ctx, s.key_ProjectInfo(projectId)).Result()
	if err != nil {
		return errs.NewInternalError(
			fmt.Errorf("failed to get project: project_id=%s, err=%w", projectId, err))
	}

	if n == 0 {
		return errs.NewNotFound(err,
			fmt.Sprintf("project not found: id=%s", projectId))
	}

	//* prepare pipeline
	pipe := s.client.TxPipeline()

	pipe.ZRem(ctx, s.key_ProjectEndpoints(projectId), endpointId)
	pipe.Del(ctx, s.key_EndpointInfo(projectId, endpointId))
	pipe.Del(ctx, s.key_EndpointStatus(projectId, endpointId))

	//* execute
	_, err = pipe.Exec(ctx)
	if err != nil {
		return errs.NewInternalError(fmt.Errorf("project_id=%s, endpoint_id=%s, err=%w", projectId, endpointId, err))
	}

	return nil
}
