// RECONSTRUCTION — not original source. See ../../REVERSE-ENGINEERING.md.
//
// Type NAMES and JSON field names are [E] evidenced (pclntab type symbols +
// string constants + the backend registration payload in
// ppanel-server/internal/report/tool.go). Field GO types and any field not
// listed in the recovered contract are [I] inferred and marked inline.
package types

// RegisterServiceRequest is the POST /basic/register body sent by a module
// (e.g. ppanel-server) to announce itself to the gateway. Field names [E];
// Go types [I].
type RegisterServiceRequest struct {
	Secret         string `json:"secret"`
	ServiceName    string `json:"service_name"`
	ServiceURL     string `json:"service_url"`
	ProxyPath      string `json:"proxy_path"`
	HeartbeatURL   string `json:"heartbeat_url"`
	Repository     string `json:"repository"`
	ServiceVersion string `json:"service_version"`
}

// RegisterServiceResponse acknowledges a registration. Shape [U] unrecovered;
// an empty-but-present success envelope is the minimal compatible form.
type RegisterServiceResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message,omitempty"`
}

// HeartbeatRequest identifies which registered service is reporting liveness.
// Keyed by service_name [I].
type HeartbeatRequest struct {
	ServiceName string `json:"service_name"`
}

// CheckServiceVersionRequest asks the gateway for current vs latest version of
// a registered service.
type CheckServiceVersionRequest struct {
	ServiceName string `json:"service_name"`
}

// ServiceVersionResponse reports installed and available versions. Field set
// [I] inferred from CheckServiceVersion + GitHub release lookup symbols.
type ServiceVersionResponse struct {
	ServiceName    string `json:"service_name"`
	CurrentVersion string `json:"current_version"`
	LatestVersion  string `json:"latest_version"`
	UpdateNeeded   bool   `json:"update_needed"`
}

// UpdateServiceRequest triggers an update of a registered service to a target
// (or latest) version.
type UpdateServiceRequest struct {
	Secret      string `json:"secret"`
	ServiceName string `json:"service_name"`
	Version     string `json:"version,omitempty"` // [I] optional target; empty => latest
}

// UpdateServiceResponse reports the outcome of an update attempt.
type UpdateServiceResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message,omitempty"`
}
