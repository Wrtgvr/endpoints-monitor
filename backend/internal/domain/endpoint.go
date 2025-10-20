package domain

type Endpoint struct {
	ID           string
	Name         string
	URL          string
	Status       string
	LastChecked  string
	ResponseTime string
	ProjectId    string
}

type EndpointInfo struct {
	ID        string
	Name      string
	URL       string
	ProjectId string
}

type EndpointStatus struct {
	ID           string
	Status       string
	LastChecked  string
	ResponseTime string
}
