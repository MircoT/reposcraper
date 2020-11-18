package cmd

// GitLabRepo struct to unpack from GitLab response
type GitLabRepo struct {
	Name        string `json:"name"`
	WebURL      string `json:"web_url"`
	Description string `json:"description"`
	Visibility  string `json:"visibility"`
}

func (r GitLabRepo) GetName() string {
	return r.Name
}

func (r GitLabRepo) GetDescription() string {
	return r.Description
}

func (r GitLabRepo) GetURL() string {
	return r.WebURL
}

func (r GitLabRepo) GetVisibility() string {
	return r.Visibility
}

func (r GitLabRepo) GetType() string {
	return "GitLab"
}
