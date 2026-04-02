package handler

// StatusResponse is the standard response for status-only endpoints.
type StatusResponse struct {
	Status string `json:"status"`
}
