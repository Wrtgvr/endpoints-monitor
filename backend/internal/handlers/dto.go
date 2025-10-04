package handlers

//* Request
type CreateEndpointRequest struct {
	Name string `json:"name"`
	URL  string `json:"url"`
}

type UpdateEndpointInfoRequest struct {
	Name string `json:"name"`
	URL  string `json:"url"`
}

//* Response
type EndpointResponse struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	URL          string `json:"url"`
	Status       string `json:"status"`
	LastChecked  string `json:"last_checked_at"`
	ResponseTime string `json:"response_time"`
}

type EndpointInfoResponse struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	URL  string `json:"url"`
}

type EndpointStatusResponse struct {
	ID           string `json:"id"`
	Status       string `json:"status"`
	LastChecked  string `json:"last_checked_at"`
	ResponseTime string `json:"response_time"`
}
