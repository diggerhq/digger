package envprovider

import (
	"testing"

	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/stretchr/testify/assert"
)

func TestRetrieve(t *testing.T) {

	tests := map[string]struct {
		key    string
		secret string
	}{
		"digger prefix": {key: "DIGGER_AWS_ACCESS_KEY_ID", secret: "DIGGER_AWS_SECRET_ACCESS_KEY"},
		"no prefix":     {key: "AWS_ACCESS_KEY_ID", secret: "AWS_SECRET_ACCESS_KEY"},
		"other":         {key: "AWS_ACCESS_KEY", secret: "AWS_SECRET_KEY"},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Setenv(tc.key, "key")
			t.Setenv(tc.secret, "secret")
			e := EnvProvider{}
			act, _ := e.Retrieve()
			exp := credentials.Value(credentials.Value{AccessKeyID: "key", SecretAccessKey: "secret", SessionToken: "", ProviderName: "DiggerEnvProvider"})
			assert.Equal(t, exp, act)
		})
	}
}
