package session

// refreshRequest is the JSON body for the token refresh endpoint.
type refreshRequest struct {
	RefreshToken string `json:"refresh_token" validate:"required"`
}

// tokenResponse is returned on successful token refresh.
type tokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token,omitempty"`
	ExpiresIn    int    `json:"expires_in"`
}

// sessionResponse represents an active session in list responses.
type sessionResponse struct {
	ID         string `json:"id"`
	IPAddress  string `json:"ip_address,omitempty"`
	DeviceName string `json:"device_name,omitempty"`
	Country    string `json:"country,omitempty"`
	LastActive string `json:"last_active_at"`
	CreatedAt  string `json:"created_at"`
}
