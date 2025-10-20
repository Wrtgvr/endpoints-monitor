package storage

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"

	"github.com/redis/go-redis/v9"
	"github.com/wrtgvr/websites-monitor/internal/config"
	"github.com/wrtgvr/websites-monitor/internal/domain"
	errs "github.com/wrtgvr/websites-monitor/internal/errors"
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

func (s *RedisStorage) CreateProject(ctx context.Context, projectInfo *domain.Project) *errs.AppError {
	//* check if admin api key is already exists
	exists, err := s.client.Exists(ctx, s.key_ProjectByAdminAPIKey(projectInfo.AdminKey.Key)).Result()
	if err != nil {
		return errs.NewInternalError(fmt.Errorf("failed to check api key existence: %w", err))
	}
	if exists > 0 {
		return errs.NewConflict(nil, "api key already exists")
	}

	//* prepare pipeline
	pipe := s.client.TxPipeline()

	s.setNewProjectInfo_AddToPipe(ctx, pipe, projectInfo)
	s.setNewAdminKey_AddToPipe(ctx, pipe, projectInfo.AdminKey)

	//* execute pipeline
	_, err = pipe.Exec(ctx)
	if err != nil {
		return errs.NewInternalError(fmt.Errorf("failed to create a project, err=%w", err))
	}

	return nil
}

func (s *RedisStorage) GetProjectIDByAPIKey(ctx context.Context, apiKey string) (string, *errs.AppError) {
	id, err := s.client.HGet(ctx, s.key_ProjectByAdminAPIKey(apiKey), AdminAPIKey_HSet_ProjectID).Result()
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
	//* prepare pipeline
	projectInfo, err := s.client.HGetAll(ctx, s.key_ProjectInfo(projectId)).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, errs.NewNotFound(err, fmt.Sprintf("project not found: project_id=%s", projectId))
		}
		return nil, errs.NewInternalError(err)
	}

	keyInfo, err := s.client.HGetAll(ctx, s.key_ProjectKeyInfo(projectId, projectInfo[Project_HSet_AdminKey])).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, errs.NewNotFound(err, fmt.Sprintf("admin key not found: project_id=%s", projectId))
		}
		return nil, errs.NewInternalError(err)
	}

	return &domain.Project{
		ID:   projectId,
		Name: projectInfo[Project_HSet_Name],
		AdminKey: &domain.APIKey{
			Key:       projectInfo[Project_HSet_AdminKey],
			Type:      domain.KeyTypeAdmin,
			ProjectID: projectId,
			CreatedAt: keyInfo[APIKey_HSet_CreatedAt],
		},
	}, nil
}

func (s *RedisStorage) ChangeProjectName(ctx context.Context, projectId, newName string) *errs.AppError {
	//* check if project exists
	n, err := s.client.Exists(ctx, s.key_ProjectInfo(projectId)).Result()
	if err != nil {
		return errs.NewInternalError(err)
	}
	if n == 0 {
		return errs.NewNotFound(nil, fmt.Sprintf("project with given id not found, project_id=%s", projectId))
	}

	//* update project info
	if err := s.client.HSet(ctx, s.key_ProjectInfo(projectId), Project_HSet_Name, newName).Err(); err != nil {
		return errs.NewInternalError(
			fmt.Errorf("failed to update project name, project_id=%s, err=%w", projectId, err))
	}
	return nil
}

func (s *RedisStorage) DeleteProject(ctx context.Context, projectId string) *errs.AppError {
	//* prepare pipeline to get data
	pipeGet := s.client.TxPipeline()
	keys, _ := pipeGet.ZRange(ctx, s.key_ProjectAPIKeys(projectId), 0, -1).Result()
	adminKey, _ := pipeGet.HGet(ctx, s.key_ProjectInfo(projectId), Project_HSet_AdminKey).Result()

	// execute
	_, err := pipeGet.Exec(ctx)
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return errs.NewNotFound(err, fmt.Sprintf("failed to get project keys: project_id=%s", projectId))
		}
		return errs.NewInternalError(err)
	}

	//* prepare pipeline to delete project data
	pipe := s.client.Pipeline()

	// delete all project keys info
	for _, k := range keys {
		pipe.Del(ctx, s.key_ProjectKeyInfo(projectId, k))
	}

	// project info
	pipe.Del(ctx,
		s.key_ProjectInfo(projectId),
		s.key_ProjectByAdminAPIKey(adminKey),
		s.key_ProjectAPIKeys(projectId))

	_, err = pipe.Exec(ctx)
	if err != nil {
		return errs.NewInternalError(err)
	}
	return nil
}

//* keys

func (s *RedisStorage) UpdateProjectAdminAPIKey(ctx context.Context, oldApiKey, newApiKey *domain.APIKey) *errs.AppError {
	//* check keys project id
	if oldApiKey.ProjectID != newApiKey.ProjectID {
		return errs.NewBadRequest(nil, "new and old admin api keys has different project ids")
	}
	projectId := newApiKey.ProjectID

	//* prepare pipeine
	pipe := s.client.Pipeline()

	s.deleteAdminKeyInfo_AddToPipe(ctx, pipe, projectId, oldApiKey.Key)
	s.setNewAdminKey_AddToPipe(ctx, pipe, newApiKey)

	//* exec
	_, err := pipe.Exec(ctx)
	if err != nil {
		return errs.NewInternalError(
			fmt.Errorf("failed to update project admin api key, id=%s, err=%s", projectId, err))
	}
	return nil
}

func (s *RedisStorage) AddReadonlyAPIKey(ctx context.Context, key *domain.APIKey) *errs.AppError {
	//* check amount of keys
	n, err := s.client.ZCard(ctx, s.key_ProjectAPIKeys(key.ProjectID)).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return errs.NewNotFound(err,
				fmt.Sprintf("project read-only api keys value not found: project_id=%s", key.ProjectID))
		}
		return errs.NewInternalError(
			fmt.Errorf("failed to get amount of zset members: project_id=%s, err=%w",
				key.ProjectID, err))
	}
	if n >= s.maxReadOnlyKeys {
		return errs.NewConflict(nil, fmt.Sprintf("A project cannot have more than %d read-only api keys", s.maxReadOnlyKeys))
	}

	//* prepare pipeline
	pipe := s.client.TxPipeline()

	s.setNewKeyInfo_AddToPipe(ctx, pipe, key)

	//* exec
	_, err = pipe.Exec(ctx)
	if err != nil {
		return errs.NewInternalError(
			fmt.Errorf("failed to add read-only api key: project_id=%s, err=%w", key.ProjectID, err))
	}

	return nil
}

func (s *RedisStorage) RemoveReadonlyAPIKey(ctx context.Context, projectId, key string) *errs.AppError {
	//* prepare pipeline
	pipe := s.client.TxPipeline()

	s.deleteKeyInfo_AddToPipe(ctx, pipe, projectId, key)

	//* exec
	_, err := pipe.Exec(ctx)
	if err != nil {
		return errs.NewInternalError(
			fmt.Errorf("failed to remove read-only key, project_id=%s, err=%w",
				projectId, err))
	}
	return nil
}

func (s *RedisStorage) GetReadonlyKeys(ctx context.Context, projectId string) ([]*domain.APIKey, *errs.AppError) {
	//* get project keys
	keys, err := s.client.ZRange(ctx, s.key_ProjectAPIKeys(projectId), 0, -1).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, errs.NewNotFound(err,
				fmt.Sprintf("project read-only api keys value not found: project_id=%s", projectId))
		}
		return nil, errs.NewInternalError(
			fmt.Errorf("failed to get read-only api keys ids: err=%w", err))
	}

	readOnlyKeys := make([]*domain.APIKey, 0)
	failedKeys := 0
	for _, key := range keys {
		keyInfo, err := s.client.HGetAll(ctx, s.key_ProjectKeyInfo(projectId, key)).Result()
		if err != nil {
			log.Printf("WARN: failed to get key info: project_id=%s, err=%v\n", projectId, err)
			failedKeys++
			continue
		}

		keyType := keyInfo[APIKey_HSet_Type]
		if keyType != domain.KeyTypeReadOnly && keyType != domain.KeyTypeAdmin {
			log.Printf("WARN: key has unknown type: key_type=%s\n", keyType)
			failedKeys++
			continue
		}
		readOnlyKeys = append(readOnlyKeys, &domain.APIKey{
			Key:       key,
			ProjectID: projectId,
			Type:      keyType,
			CreatedAt: keyInfo[APIKey_HSet_CreatedAt],
		})
	}

	if failedKeys > 0 {
		log.Printf("WARN: failed to load %d endpoints: project_id=%s\n", failedKeys, projectId)
	}

	return readOnlyKeys, nil
}

func (s *RedisStorage) GetKeyInfo(ctx context.Context, projectId, key string) (*domain.APIKey, *errs.AppError) {
	keyInfo, err := s.client.HGetAll(ctx, s.key_ProjectKeyInfo(projectId, key)).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, errs.NewNotFound(err,
				fmt.Sprintf("project read-only api keys value not found: project_id=%s", projectId))
		}
		return nil, errs.NewInternalError(
			fmt.Errorf("failed to get read-only api keys ids: err=%w", err))
	}

	return &domain.APIKey{
		Key:       key,
		ProjectID: projectId,
		Type:      keyInfo[APIKey_HSet_Type],
		CreatedAt: keyInfo[APIKey_HSet_CreatedAt],
	}, nil
}

//* endpoints

func (s *RedisStorage) CreateEndpoint(ctx context.Context, endpointInfo *domain.EndpointInfo) *errs.AppError {
	//* check if project exists
	n, err := s.client.Exists(ctx, s.key_ProjectInfo(endpointInfo.ProjectId)).Result()
	if err != nil {
		return errs.NewInternalError(
			fmt.Errorf("failed to get project: project_id=%s, err=%w", endpointInfo.ProjectId, err))
	}
	if n == 0 {
		return errs.NewNotFound(err,
			fmt.Sprintf("project not found: id=%s", endpointInfo.ProjectId))
	}

	//* check amount of keys
	n, err = s.client.ZCard(ctx, s.key_ProjectEndpoints(endpointInfo.ProjectId)).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return errs.NewNotFound(err,
				fmt.Sprintf("project endpoints value not found: project_id=%s", endpointInfo.ProjectId))
		}
		return errs.NewInternalError(
			fmt.Errorf("failed to get amount of zset members: project_id=%s, err=%w",
				endpointInfo.ProjectId, err))
	}
	if n >= s.maxEndpoints {
		return errs.NewConflict(nil, fmt.Sprintf("A project cannot have more than %d endpoints", s.maxReadOnlyKeys))
	}

	//* prepare pipeline
	pipe := s.client.TxPipeline()

	s.setNewEndpoint_AddToPipe(ctx, pipe, endpointInfo)

	_, err = pipe.Exec(ctx)
	if err != nil {
		errs.NewInternalError(fmt.Errorf("failed to create new endpoint: project_id=%s, err=%w", endpointInfo.ProjectId, err))
	}

	return nil
}

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
		if info[EndpointInfo_HSet_Url] == "" {
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
			Name:         info[EndpointInfo_HSet_Name],
			URL:          info[EndpointInfo_HSet_Url],
			ProjectId:    info[EndpointInfo_HSet_ProjectId],
			Status:       status[EndpointStatus_HSet_Status],
			LastChecked:  status[EndpointStatus_HSet_LastChecked],
			ResponseTime: status[EndpointStatus_HSet_ResponseTime],
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
	failedEndpoints := 0
	for _, id := range ids {
		info, err := infoCmds[id].Result()
		if err != nil {
			log.Printf("WARN: failed to get endpoint, id=%s, err=%v\n", id, err)
			failedEndpoints++
			continue
		}
		// check for required field
		if info[EndpointInfo_HSet_Url] == "" {
			log.Printf("WARN: endpoint does not have required URL field, id=%s, err=%v\n", id, err)
			failedEndpoints++
			continue
		}
		endpoints = append(endpoints, &domain.EndpointInfo{
			ID:        id,
			Name:      info[EndpointInfo_HSet_Name],
			URL:       info[EndpointInfo_HSet_Url],
			ProjectId: info[EndpointInfo_HSet_ProjectId],
		})
	}

	if failedEndpoints > 0 {
		log.Printf("WARN: failed to load %d endpoints\n", failedEndpoints)
	}

	return endpoints, nil
}

func (s *RedisStorage) UpdateEndpointInfo(ctx context.Context, ep *domain.EndpointInfo) *errs.AppError {
	//* get current url and name
	res, err := s.client.HGetAll(ctx, s.key_EndpointInfo(ep.ProjectId, ep.ID)).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return errs.NewNotFound(err,
				fmt.Sprintf("endpoint not found: id=%s", ep.ID))
		}
		return errs.NewInternalError(
			fmt.Errorf("failed to get endpoint info: id=%s, err=%w", ep.ID, err))
	}

	//* if url or name from given info struct is nil value ("") then use enpodint current url/name
	url := ep.URL
	if url == "" {
		url = res[EndpointInfo_HSet_Url]
	}
	name := ep.Name
	if name == "" {
		name = res[EndpointInfo_HSet_Name]
	}

	//* update endpoint
	err = s.client.HSet(ctx, s.key_EndpointInfo(ep.ProjectId, ep.ID),
		EndpointInfo_HSet_Url, url,
		EndpointInfo_HSet_Name, name,
	).Err()
	if err != nil {
		return errs.NewInternalError(
			fmt.Errorf("failed to update endpoint info: req_endpoint_id=%s, req_new_endpoint_name=%s, req_new_endpoint_url=%s, err=%w", ep.ID, ep.Name, ep.URL, err))
	}

	return nil
}

func (s *RedisStorage) UpdateEndpointStatus(ctx context.Context, projectId string, ep *domain.EndpointStatus) *errs.AppError {
	//* check if endpoint exists
	n, err := s.client.Exists(ctx, s.key_EndpointInfo(projectId, ep.ID)).Result()
	if err != nil {
		return errs.NewInternalError(
			fmt.Errorf("failed to get endpoint info: id=%s, err=%w", ep.ID, err))
	}

	if n == 0 {
		return errs.NewNotFound(err,
			fmt.Sprintf("endpoint not found: id=%s", ep.ID))
	}

	//* update endpoint status
	err = s.client.HSet(ctx, s.key_EndpointInfo(projectId, ep.ID),
		EndpointStatus_HSet_Status, ep.Status,
		EndpointStatus_HSet_LastChecked, ep.LastChecked,
		EndpointStatus_HSet_ResponseTime, ep.ResponseTime,
	).Err()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return errs.NewAppError(err, fmt.Sprintf("endpoint with not found id=%s", ep.ID), errs.TypeNotFound, http.StatusNotFound)
		}
		return errs.NewInternalError(fmt.Errorf("id=%s, err=%w", ep.ID, err))
	}
	return nil
}

func (s *RedisStorage) DeleteEndpoint(ctx context.Context, projectId, endpointId string) *errs.AppError {
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

	s.deleteEndpoint_AddToPipe(ctx, pipe, endpointId, projectId)

	//* execute
	_, err = pipe.Exec(ctx)
	if err != nil {
		return errs.NewInternalError(fmt.Errorf("project_id=%s, endpoint_id=%s, err=%w", projectId, endpointId, err))
	}

	return nil
}
