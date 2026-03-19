package github

// Repo represents a Github repository.
type Repo struct {
	ID            int64
	FullName      string
	DefaultBranch string
}

// ClonedRepo represents a github repository with the CloneURL.
type ClonedRepo struct {
	ID       int64  `json:"id"`
	FullName string `json:"full_name"`
	CloneURL string `json:"clone_url"`
}

// Installation represents a Github installation struct with an app ID.
type Installation struct {
	ID int64 `json:"id"`
}

// PushPayload is the minimal GitHub webhook payload structure we need.
type PushPayload struct {
	Ref          string       `json:"ref"`
	After        string       `json:"after"`
	Action       string       `json:"action"`
	Repository   ClonedRepo   `json:"repository"`
	Installation Installation `json:"installation"`
}
