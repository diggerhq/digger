package models

import "time"

type Event interface{}

type Base struct {
	Ref string `json:"ref"`
}

type EditSHA struct {
	From *string `json:"from,omitempty"`
}

type Comment struct {
	Body string `json:"body"`
}

type Installation struct {
	ID                     *int64     `json:"id,omitempty"`
	NodeID                 *string    `json:"node_id,omitempty"`
	AppID                  *int64     `json:"app_id,omitempty"`
	AppSlug                *string    `json:"app_slug,omitempty"`
	TargetID               *int64     `json:"target_id,omitempty"`
	Account                *User      `json:"account,omitempty"`
	AccessTokensURL        *string    `json:"access_tokens_url,omitempty"`
	RepositoriesURL        *string    `json:"repositories_url,omitempty"`
	HTMLURL                *string    `json:"html_url,omitempty"`
	TargetType             *string    `json:"target_type,omitempty"`
	SingleFileName         *string    `json:"single_file_name,omitempty"`
	RepositorySelection    *string    `json:"repository_selection,omitempty"`
	Events                 []string   `json:"events,omitempty"`
	SingleFilePaths        []string   `json:"single_file_paths,omitempty"`
	CreatedAt              *Timestamp `json:"created_at,omitempty"`
	UpdatedAt              *Timestamp `json:"updated_at,omitempty"`
	HasMultipleSingleFiles *bool      `json:"has_multiple_single_files,omitempty"`
	SuspendedBy            *User      `json:"suspended_by,omitempty"`
	SuspendedAt            *Timestamp `json:"suspended_at,omitempty"`
}

type Issue struct {
	Number int `json:"number"`
}

type IssueComment struct {
	ID        *int64     `json:"id,omitempty"`
	NodeID    *string    `json:"node_id,omitempty"`
	Body      *string    `json:"body,omitempty"`
	User      *User      `json:"user,omitempty"`
	Reactions *Reactions `json:"reactions,omitempty"`
	CreatedAt *Timestamp `json:"created_at,omitempty"`
	UpdatedAt *Timestamp `json:"updated_at,omitempty"`
	// AuthorAssociation is the comment author's relationship to the issue's repository.
	// Possible values are "COLLABORATOR", "CONTRIBUTOR", "FIRST_TIMER", "FIRST_TIME_CONTRIBUTOR", "MEMBER", "OWNER", or "NONE".
	AuthorAssociation *string `json:"author_association,omitempty"`
	URL               *string `json:"url,omitempty"`
	HTMLURL           *string `json:"html_url,omitempty"`
	IssueURL          *string `json:"issue_url,omitempty"`
}

type Label struct {
	ID          *int64  `json:"id,omitempty"`
	URL         *string `json:"url,omitempty"`
	Name        *string `json:"name,omitempty"`
	Color       *string `json:"color,omitempty"`
	Description *string `json:"description,omitempty"`
	Default     *bool   `json:"default,omitempty"`
	NodeID      *string `json:"node_id,omitempty"`
}

// Organization represents a GitHub organization account.
type Organization struct {
	Login             *string    `json:"login,omitempty"`
	ID                *int64     `json:"id,omitempty"`
	AvatarURL         *string    `json:"avatar_url,omitempty"`
	Name              *string    `json:"name,omitempty"`
	Company           *string    `json:"company,omitempty"`
	Blog              *string    `json:"blog,omitempty"`
	Location          *string    `json:"location,omitempty"`
	Email             *string    `json:"email,omitempty"`
	TwitterUsername   *string    `json:"twitter_username,omitempty"`
	Description       *string    `json:"description,omitempty"`
	PublicRepos       *int       `json:"public_repos,omitempty"`
	PublicGists       *int       `json:"public_gists,omitempty"`
	Followers         *int       `json:"followers,omitempty"`
	Following         *int       `json:"following,omitempty"`
	CreatedAt         *Timestamp `json:"created_at,omitempty"`
	UpdatedAt         *Timestamp `json:"updated_at,omitempty"`
	TotalPrivateRepos *int64     `json:"total_private_repos,omitempty"`
	OwnedPrivateRepos *int64     `json:"owned_private_repos,omitempty"`
	PrivateGists      *int       `json:"private_gists,omitempty"`

	// DefaultRepoPermission can be one of: "read", "write", "admin", or "none". (Default: "read").
	// It is only used in OrganizationsService.Edit.
	DefaultRepoPermission *string `json:"default_repository_permission,omitempty"`
	// DefaultRepoSettings can be one of: "read", "write", "admin", or "none". (Default: "read").
	// It is only used in OrganizationsService.Get.
	DefaultRepoSettings *string `json:"default_repository_settings,omitempty"`

	// MembersCanCreateRepos default value is true and is only used in Organizations.Edit.
	MembersCanCreateRepos *bool `json:"members_can_create_repositories,omitempty"`

	// https://developer.github.com/changes/2019-12-03-internal-visibility-changes/#rest-v3-api
	MembersCanCreatePublicRepos   *bool `json:"members_can_create_public_repositories,omitempty"`
	MembersCanCreatePrivateRepos  *bool `json:"members_can_create_private_repositories,omitempty"`
	MembersCanCreateInternalRepos *bool `json:"members_can_create_internal_repositories,omitempty"`

	// MembersCanForkPrivateRepos toggles whether organization members can fork private organization repositories.
	MembersCanForkPrivateRepos *bool `json:"members_can_fork_private_repositories,omitempty"`

	// MembersAllowedRepositoryCreationType denotes if organization members can create repositories
	// and the type of repositories they can create. Possible values are: "all", "private", or "none".
	//
	// Deprecated: Use MembersCanCreatePublicRepos, MembersCanCreatePrivateRepos, MembersCanCreateInternalRepos
	// instead. The new fields overrides the existing MembersAllowedRepositoryCreationType during 'edit'
	// operation and does not consider 'internal' repositories during 'get' operation
	MembersAllowedRepositoryCreationType *string `json:"members_allowed_repository_creation_type,omitempty"`

	// API URLs
	URL              *string `json:"url,omitempty"`
	EventsURL        *string `json:"events_url,omitempty"`
	HooksURL         *string `json:"hooks_url,omitempty"`
	IssuesURL        *string `json:"issues_url,omitempty"`
	MembersURL       *string `json:"members_url,omitempty"`
	PublicMembersURL *string `json:"public_members_url,omitempty"`
	ReposURL         *string `json:"repos_url,omitempty"`
}

type OwnerInfo struct {
	User *User `json:"user,omitempty"`
	Org  *User `json:"organization,omitempty"`
}

type Repository struct {
	DefaultBranch string `json:"default_branch"`
}

type Team struct {
	ID              *int64          `json:"id,omitempty"`
	NodeID          *string         `json:"node_id,omitempty"`
	Name            *string         `json:"name,omitempty"`
	Description     *string         `json:"description,omitempty"`
	URL             *string         `json:"url,omitempty"`
	Slug            *string         `json:"slug,omitempty"`
	Permission      *string         `json:"permission,omitempty"`
	Permissions     map[string]bool `json:"permissions,omitempty"`
	Privacy         *string         `json:"privacy,omitempty"`
	MembersCount    *int            `json:"members_count,omitempty"`
	ReposCount      *int            `json:"repos_count,omitempty"`
	Organization    *Organization   `json:"organization,omitempty"`
	HTMLURL         *string         `json:"html_url,omitempty"`
	MembersURL      *string         `json:"members_url,omitempty"`
	RepositoriesURL *string         `json:"repositories_url,omitempty"`
}
type User struct {
	Login           *string    `json:"login,omitempty"`
	ID              *int64     `json:"id,omitempty"`
	AvatarURL       *string    `json:"avatar_url,omitempty"`
	Name            *string    `json:"name,omitempty"`
	Company         *string    `json:"company,omitempty"`
	Blog            *string    `json:"blog,omitempty"`
	Location        *string    `json:"location,omitempty"`
	Email           *string    `json:"email,omitempty"`
	TwitterUsername *string    `json:"twitter_username,omitempty"`
	PublicRepos     *int       `json:"public_repos,omitempty"`
	PublicGists     *int       `json:"public_gists,omitempty"`
	Followers       *int       `json:"followers,omitempty"`
	Following       *int       `json:"following,omitempty"`
	CreatedAt       *Timestamp `json:"created_at,omitempty"`
	UpdatedAt       *Timestamp `json:"updated_at,omitempty"`

	// API URLs
	URL               *string `json:"url,omitempty"`
	EventsURL         *string `json:"events_url,omitempty"`
	FollowingURL      *string `json:"following_url,omitempty"`
	FollowersURL      *string `json:"followers_url,omitempty"`
	GistsURL          *string `json:"gists_url,omitempty"`
	OrganizationsURL  *string `json:"organizations_url,omitempty"`
	ReceivedEventsURL *string `json:"received_events_url,omitempty"`
	ReposURL          *string `json:"repos_url,omitempty"`
	StarredURL        *string `json:"starred_url,omitempty"`
	SubscriptionsURL  *string `json:"subscriptions_url,omitempty"`

	// Permissions and RoleName identify the permissions and role that a user has on a given
	// repository. These are only populated when calling Repositories.ListCollaborators.
	Permissions map[string]bool `json:"permissions,omitempty"`
	RoleName    *string         `json:"role_name,omitempty"`
}

type Reactions struct {
	TotalCount *int    `json:"total_count,omitempty"`
	PlusOne    *int    `json:"+1,omitempty"`
	MinusOne   *int    `json:"-1,omitempty"`
	Laugh      *int    `json:"laugh,omitempty"`
	Confused   *int    `json:"confused,omitempty"`
	Heart      *int    `json:"heart,omitempty"`
	Hooray     *int    `json:"hooray,omitempty"`
	Rocket     *int    `json:"rocket,omitempty"`
	Eyes       *int    `json:"eyes,omitempty"`
	URL        *string `json:"url,omitempty"`
}

type RepoName struct {
	From *string `json:"from,omitempty"`
}

type Timestamp struct {
	time.Time
}
