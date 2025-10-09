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
	return fmt.Sprintf("project_api_key:%s", apiKey)
}

// ZSet
func (s RedisStorage) key_ProjectsReadOnlyAPIKeys(projectId string) string {
	return fmt.Sprintf("project_readonly_keys:%s", projectId)
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
