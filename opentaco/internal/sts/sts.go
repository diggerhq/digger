package sts

// Package sts will issue short-lived S3 credentials (stateless or stateful).
// This file scaffolds the public surface for later implementation.

type Issuer interface {
    // Issue returns AWS Process Credentials JSON fields.
    Issue(subject string) (AccessKeyID, SecretAccessKey, SessionToken string, expirationUnix int64, err error)
}

