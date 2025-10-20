package storage

// HSet Keys
const (
	//* project

	// Project HSet field for project name.
	Project_HSet_Name = "name"
	// Project HSet field for project admin key.
	Project_HSet_AdminKey = "admin_key"

	//* api key

	// Admin api key HSet field for project id.
	AdminAPIKey_HSet_ProjectID = "project_id"
	// API key HSet field for key type.
	APIKey_HSet_Type = "type"
	// API key HSet field for key creation date.
	APIKey_HSet_CreatedAt = "created_at"

	//* endpoint
	// Endpoint info HSet field for name
	EndpointInfo_HSet_Name = "name"
	// Endpoint info HSet field for url
	EndpointInfo_HSet_Url = "url"
	// Endpoint info HSet field for project id
	EndpointInfo_HSet_ProjectId = "project_id"
	// Endpoint status HSet field for status
	EndpointStatus_HSet_Status = "status"
	// Endpoint status HSet field for last checked time
	EndpointStatus_HSet_LastChecked = "last_checked"
	// Endpoint status HSet field for response time
	EndpointStatus_HSet_ResponseTime = "response_time"
)
