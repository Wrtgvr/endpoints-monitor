package storage

import (
	"context"
	"time"

	redis "github.com/redis/go-redis/v9"
	"github.com/wrtgvr/websites-monitor/internal/domain"
)

// * project
func (s *RedisStorage) setNewProjectInfo_AddToPipe(ctx context.Context, pipe redis.Pipeliner, project *domain.Project) {
	pipe.HSet(ctx, s.key_ProjectInfo(project.ID),
		Project_HSet_Name, project.Name,
		Project_HSet_AdminKey, project.AdminKey.Key,
	)
}

// * api key
func (s *RedisStorage) setNewAdminKey_AddToPipe(ctx context.Context, pipe redis.Pipeliner, adminKey *domain.APIKey) {
	pipe.HSet(ctx, s.key_ProjectByAdminAPIKey(adminKey.Key),
		AdminAPIKey_HSet_ProjectID, adminKey.ProjectID)
	s.setNewKeyInfo_AddToPipe(ctx, pipe, adminKey)
}

func (s *RedisStorage) setNewKeyInfo_AddToPipe(ctx context.Context, pipe redis.Pipeliner, key *domain.APIKey) {
	pipe.HSet(ctx, s.key_ProjectKeyInfo(key.ProjectID, key.Key),
		APIKey_HSet_Type, key.Type,
		APIKey_HSet_CreatedAt, time.Now().Format(time.RFC3339))
	pipe.ZAdd(ctx, s.key_ProjectAPIKeys(key.ProjectID), redis.Z{
		Score:  float64(time.Now().Unix()),
		Member: key.Key,
	})
}

func (s *RedisStorage) deleteAdminKeyInfo_AddToPipe(ctx context.Context, pipe redis.Pipeliner, projectId, key string) {
	pipe.Del(ctx, s.key_ProjectByAdminAPIKey(key))
	s.deleteKeyInfo_AddToPipe(ctx, pipe, projectId, key)
}

func (s *RedisStorage) deleteKeyInfo_AddToPipe(ctx context.Context, pipe redis.Pipeliner, projectId, key string) {
	pipe.Del(ctx, s.key_ProjectKeyInfo(projectId, key))
	pipe.ZRem(ctx, s.key_ProjectAPIKeys(projectId), key)
}

// * endpoints
func (s *RedisStorage) setNewEndpoint_AddToPipe(ctx context.Context, pipe redis.Pipeliner, ep *domain.EndpointInfo) {
	pipe.ZAdd(ctx, s.key_ProjectEndpoints(ep.ProjectId), redis.Z{
		Score:  float64(time.Now().Unix()),
		Member: ep.ID,
	})
	pipe.HSet(ctx, s.key_EndpointInfo(ep.ProjectId, ep.ID),
		EndpointInfo_HSet_Name, ep.Name,
		EndpointInfo_HSet_Url, ep.URL,
		EndpointInfo_HSet_ProjectId, ep.ProjectId)
	pipe.HSet(ctx, s.key_EndpointStatus(ep.ProjectId, ep.ID),
		EndpointStatus_HSet_Status, "Unknown",
		EndpointStatus_HSet_LastChecked, "Never",
		EndpointStatus_HSet_ResponseTime, "0")
}

func (s *RedisStorage) deleteEndpoint_AddToPipe(ctx context.Context, pipe redis.Pipeliner, endpointId, projectId string) {
	pipe.ZRem(ctx, s.key_ProjectEndpoints(projectId), endpointId)
	pipe.Del(ctx, s.key_EndpointInfo(projectId, endpointId))
	pipe.Del(ctx, s.key_EndpointStatus(projectId, endpointId))
}
