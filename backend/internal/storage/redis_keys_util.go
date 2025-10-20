package storage

import "fmt"

//* projects

// HSet
func (s RedisStorage) key_ProjectInfo(projectId string) string {
	return fmt.Sprintf("projects:%s", projectId)
}

//* keys

// HSet
func (s RedisStorage) key_ProjectByAdminAPIKey(apiKey string) string {
	return fmt.Sprintf("admin_key:%s:project_id", apiKey)
}

// HSet
func (s RedisStorage) key_ProjectKeyInfo(projectId, apiKey string) string {
	return fmt.Sprintf("project:%s:keys:%s", projectId, apiKey)
}

// ZSet
func (s RedisStorage) key_ProjectAPIKeys(projectId string) string {
	return fmt.Sprintf("project:%s:keys", projectId)
}

//* endpoints

// ZSet
func (s RedisStorage) key_ProjectEndpoints(projectId string) string {
	return fmt.Sprintf("endpoints:%s", projectId)
}

// HSet
func (s RedisStorage) key_EndpointInfo(projectId, endpointId string) string {
	return fmt.Sprintf("endpoints:%s:%s:info", projectId, endpointId)
}

// HSet
func (s RedisStorage) key_EndpointStatus(projectId, endpointId string) string {
	return fmt.Sprintf("endpoints:%s:%s:status", projectId, endpointId)
}
