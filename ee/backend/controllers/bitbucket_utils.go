package controllers

import "time"

// Common structures used across different event types

type Actor struct {
	AccountID   string `json:"account_id"`
	DisplayName string `json:"display_name"`
	Nickname    string `json:"nickname,omitempty"`
	UUID        string `json:"uuid"`
	Type        string `json:"type"`
	Links       Links  `json:"links"`
}

type Links struct {
	Self     LinkItem    `json:"self"`
	HTML     LinkItem    `json:"html"`
	Avatar   LinkItem    `json:"avatar,omitempty"`
	Branches *LinkItem   `json:"branches,omitempty"`
	Commits  *LinkItem   `json:"commits,omitempty"`
	Clone    []CloneLink `json:"clone,omitempty"`
}

type LinkItem struct {
	Href string `json:"href"`
}

type CloneLink struct {
	Href string `json:"href"`
	Name string `json:"name"`
}

type Repository struct {
	Type        string    `json:"type"`
	Name        string    `json:"name"`
	FullName    string    `json:"full_name"`
	UUID        string    `json:"uuid"`
	IsPrivate   bool      `json:"is_private"`
	Owner       Owner     `json:"owner"`
	Website     string    `json:"website,omitempty"`
	SCM         string    `json:"scm"`
	Description string    `json:"description,omitempty"`
	Links       Links     `json:"links"`
	Project     Project   `json:"project,omitempty"`
	ForkPolicy  string    `json:"fork_policy,omitempty"`
	CreatedOn   time.Time `json:"created_on,omitempty"`
	UpdatedOn   time.Time `json:"updated_on,omitempty"`
	Size        int       `json:"size,omitempty"`
	Language    string    `json:"language,omitempty"`
	HasIssues   bool      `json:"has_issues,omitempty"`
	HasWiki     bool      `json:"has_wiki,omitempty"`
	MainBranch  *Branch   `json:"mainbranch,omitempty"`
}

type Owner struct {
	Username    string `json:"username"`
	DisplayName string `json:"display_name"`
	AccountID   string `json:"account_id,omitempty"`
	UUID        string `json:"uuid"`
	Type        string `json:"type"`
	Links       Links  `json:"links"`
}

type Project struct {
	Key         string    `json:"key"`
	UUID        string    `json:"uuid"`
	Name        string    `json:"name"`
	Description string    `json:"description,omitempty"`
	Links       Links     `json:"links"`
	Type        string    `json:"type"`
	IsPrivate   bool      `json:"is_private,omitempty"`
	CreatedOn   time.Time `json:"created_on,omitempty"`
	UpdatedOn   time.Time `json:"updated_on,omitempty"`
}

type Branch struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

// -- BitbucketPushEvent (repo:push) --

type BitbucketPushEvent struct {
	Actor      Actor      `json:"actor"`
	Repository Repository `json:"repository"`
	Push       Push       `json:"push"`
}

type Push struct {
	Changes []Change `json:"changes"`
}

type Change struct {
	Forced    bool      `json:"forced"`
	Old       Reference `json:"old"`
	New       Reference `json:"new"`
	Created   bool      `json:"created"`
	Closed    bool      `json:"closed"`
	Commits   []Commit  `json:"commits"`
	Truncated bool      `json:"truncated"`
}

type Reference struct {
	Name   string     `json:"name"`
	Type   string     `json:"type"`
	Target CommitInfo `json:"target,omitempty"`
}

type CommitInfo struct {
	Hash    string    `json:"hash"`
	Author  Author    `json:"author,omitempty"`
	Message string    `json:"message"`
	Date    time.Time `json:"date"`
	Parents []Parent  `json:"parents,omitempty"`
	Type    string    `json:"type"`
	Links   Links     `json:"links"`
}

type Author struct {
	Raw  string `json:"raw"`
	User Actor  `json:"user,omitempty"`
}

type Parent struct {
	Hash  string `json:"hash"`
	Type  string `json:"type"`
	Links Links  `json:"links"`
}

type Commit struct {
	Hash    string    `json:"hash"`
	Type    string    `json:"type"`
	Message string    `json:"message"`
	Author  Author    `json:"author"`
	Links   Links     `json:"links"`
	Date    time.Time `json:"date"`
	Parents []Parent  `json:"parents,omitempty"`
}

// -- BitbucketPullRequestCreatedEvent (pullrequest:created) --

type BitbucketPullRequestCreatedEvent struct {
	Actor       Actor       `json:"actor"`
	Repository  Repository  `json:"repository"`
	PullRequest PullRequest `json:"pullrequest"`
}

type PullRequest struct {
	ID                int         `json:"id"`
	Title             string      `json:"title"`
	Description       string      `json:"description"`
	State             string      `json:"state"`
	Author            Actor       `json:"author"`
	Source            PREndpoint  `json:"source"`
	Destination       PREndpoint  `json:"destination"`
	MergeCommit       *CommitInfo `json:"merge_commit,omitempty"`
	ClosedBy          *Actor      `json:"closed_by,omitempty"`
	CreatedOn         time.Time   `json:"created_on"`
	UpdatedOn         time.Time   `json:"updated_on"`
	CommentCount      int         `json:"comment_count"`
	TaskCount         int         `json:"task_count"`
	CloseSourceBranch bool        `json:"close_source_branch"`
	Type              string      `json:"type"`
	Links             Links       `json:"links"`
	Summary           Content     `json:"summary"`
	Reviewers         []Actor     `json:"reviewers"`
	Participants      []Actor     `json:"participants"`
}

type PREndpoint struct {
	Branch     Branch     `json:"branch"`
	Commit     CommitInfo `json:"commit"`
	Repository Repository `json:"repository"`
}

type Content struct {
	Raw    string `json:"raw"`
	HTML   string `json:"html"`
	Markup string `json:"markup"`
}

// -- BitbucketCommentCreatedEvent (repo:commit_comment_created and pullrequest:comment_created) --

type BitbucketCommentCreatedEvent struct {
	Actor       Actor        `json:"actor"`
	Repository  Repository   `json:"repository"`
	Comment     Comment      `json:"comment"`
	Commit      *CommitInfo  `json:"commit,omitempty"`      // For repo:commit_comment_created
	PullRequest *PullRequest `json:"pullrequest,omitempty"` // For pullrequest:comment_created
}

type Comment struct {
	ID        int       `json:"id"`
	Content   Content   `json:"content"`
	CreatedOn time.Time `json:"created_on"`
	UpdatedOn time.Time `json:"updated_on"`
	User      Actor     `json:"user"`
	Links     Links     `json:"links"`
	Parent    *Comment  `json:"parent,omitempty"` // For replies to comments
	Inline    *Inline   `json:"inline,omitempty"` // For inline comments
}

type Inline struct {
	Path string `json:"path"`
	From *int   `json:"from,omitempty"`
	To   *int   `json:"to,omitempty"`
}
