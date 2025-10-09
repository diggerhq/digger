package tfe

type VCSRepo struct {
	owner string `json:"owner"`
	name  string `json:"name"`
}

// RepoBinding models a workspace's linkage to a VCS repository.
// JSON tags are preserved for wire compatibility; symbol names are fresh.
type RepoBinding struct {
	Branch            string  `json:"branch"`
	DisplayIdentifier string  `json:"display-identifier"`
	Identifier        VCSRepo `json:"identifier"`
	IngressSubmodules bool    `json:"ingress-submodules"`
	OAuthTokenID      string  `json:"oauth-token-id"`
	RepositoryHTTPURL string  `json:"repository-http-url"`
	TagsRegex         string  `json:"tags-regex"`
	ServiceProvider   string  `json:"service-provider"`
}

type TFEVCSRepository = RepoBinding
