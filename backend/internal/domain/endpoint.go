package domain

type Endpoint struct {
	ID           string
	Name         string
	URL          string
	Status       string
	LastChecked  string
	ResponseTime string
}

type EndpointInfo struct {
	ID   string
	Name string
	URL  string
}

type EndpointStatus struct {
	ID           string
	Status       string
	LastChecked  string
	ResponseTime string
}
