package controllers

import (
	"github.com/diggerhq/digger/backend/ci_backends"
	"github.com/diggerhq/digger/backend/utils"
)

type MainController struct {
	GithubClientProvider utils.GithubClientProvider
	CiBackendProvider    ci_backends.CiBackendProvider
}
