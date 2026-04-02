package github

// userResponse is the JSON structure returned by the GitHub user endpoint.
type userResponse struct {
	ID        int    `json:"id"         validate:"required"`
	Login     string `json:"login"      validate:"required"`
	Name      string `json:"name"`
	Email     string `json:"email"`
	AvatarURL string `json:"avatar_url"`
}

// emailResponse is a single entry from the GitHub user emails endpoint.
type emailResponse struct {
	Email    string `json:"email"    validate:"required,email"`
	Primary  bool   `json:"primary"`
	Verified bool   `json:"verified"`
}
