package utils

import "strings"

func ParseRepoNamespace(namespace string) (string, string) {
	splits := strings.Split(namespace, "/")
	SCMOrganisation := splits[0]
	SCMrepository := splits[1]
	return SCMOrganisation, SCMrepository
}
