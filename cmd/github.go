package cmd

// GitHubRepo struct to unpack from GitHub response
type GitHubRepo struct {
	Name        string `json:"name"`
	HTMLURL     string `json:"html_url"`
	Description string `json:"description"`
	Language    string `json:"language"`
	Private     bool   `json:"private"`
	Fork        bool   `json:"fork"`
}

func (r GitHubRepo) GetName() string {
	return r.Name
}

func (r GitHubRepo) GetDescription() string {
	return r.Description
}

func (r GitHubRepo) GetURL() string {
	return r.HTMLURL
}

func (r GitHubRepo) GetVisibility() string {
	if r.Private {
		return "private"
	}

	return "public"
}

func (r GitHubRepo) GetType() string {
	return "GitHub"
}
