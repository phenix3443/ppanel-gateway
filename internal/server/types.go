package server

// RegisterServiceRequest is the POST /basic/register body a module (e.g.
// ppanel-server) sends to announce itself to the gateway.
type RegisterServiceRequest struct {
	Secret         string `json:"secret"`
	ServiceName    string `json:"service_name"`
	ServiceURL     string `json:"service_url"`
	ProxyPath      string `json:"proxy_path"`
	HeartbeatURL   string `json:"heartbeat_url"`
	Repository     string `json:"repository"`
	ServiceVersion string `json:"service_version"`
}

// RegisterServiceResponse acknowledges a registration.
type RegisterServiceResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message,omitempty"`
}

// ServiceVersionResponse reports a module's installed and available versions.
type ServiceVersionResponse struct {
	ServiceName    string `json:"service_name"`
	CurrentVersion string `json:"current_version"`
	LatestVersion  string `json:"latest_version"`
	UpdateNeeded   bool   `json:"update_needed"`
}

// UpdateServiceRequest triggers an update of a registered service.
type UpdateServiceRequest struct {
	Secret      string `json:"secret"`
	ServiceName string `json:"service_name"`
	Version     string `json:"version,omitempty"` // optional target; empty => latest
}

// UpdateServiceResponse reports the outcome of an update attempt.
type UpdateServiceResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message,omitempty"`
}
