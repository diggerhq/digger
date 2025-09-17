package sts

// Package sts issues short-lived S3 credentials (stateless or stateful).

type Issuer interface {
    // Issue returns AWS Process Credentials JSON fields.
    Issue(subject string, sessionToken string) (AccessKeyID, SecretAccessKey, SessionToken string, expirationUnix int64, err error)
}
