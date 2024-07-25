package dbmodels

type GithubAppInstallStatus int

const (
	GithubAppInstallActive  GithubAppInstallStatus = 1
	GithubAppInstallDeleted GithubAppInstallStatus = 2
)

type GithubAppInstallationLinkStatus int8

const (
	GithubAppInstallationLinkActive   GithubAppInstallationLinkStatus = 1
	GithubAppInstallationLinkInactive GithubAppInstallationLinkStatus = 2
)
