package google

// userInfoResponse is the JSON structure returned by the Google userinfo endpoint.
type userInfoResponse struct {
	Sub           string `json:"sub"            validate:"required"`
	Email         string `json:"email"          validate:"required,email"`
	EmailVerified bool   `json:"email_verified"`
	Name          string `json:"name"`
	Picture       string `json:"picture"`
}
