package cmd

const (
	bitBucketAccessTokenURL = "https://bitbucket.org/site/oauth2/access_token" //nolint:gosec
)

// BitbucketResp struct to unpack from Bitbucket response
type BitbucketResp struct {
	Pagelen int             `json="pagelen"`
	Values  []BitbucketRepo `json="values"`
	Page    int             `json="page"`
	Size    int             `json="size"`
	Next    string          `json="next"`
}

type BitbucketRepo struct {
	Name  string `json:"name"`
	Links struct {
		HTML struct {
			HREF string `json:"href"`
		} `json:"html"`
	} `json:"links"`
	Description string `json:"description"`
	IsPrivate   bool   `json:"is_private"`
}

func (r BitbucketRepo) GetName() string {
	return r.Name
}

func (r BitbucketRepo) GetDescription() string {
	return r.Description
}

func (r BitbucketRepo) GetURL() string {
	return r.Links.HTML.HREF
}

func (r BitbucketRepo) GetVisibility() string {
	if r.IsPrivate {
		return "private"
	}

	return "public"
}

func (r BitbucketRepo) GetType() string {
	return "Bitbucket"
}

// BitbucketToken struct used for the token request
type BitbucketToken struct {
	Scopes       string `json:"scopes"`
	AccessToken  string `json:"access_token"`
	ExpiresIn    int    `json:"expires_in"`
	TokenType    string `json:"token_type"`
	State        string `json:"state"`
	RefreshToken string `json:"refresh_token"`
}
