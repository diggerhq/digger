package envprovider

import (
	"context"
	"os"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/stretchr/testify/assert"
)

func TestRetrieve(t *testing.T) {
	t.Parallel()
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
			os.Setenv(tc.key, "key")
			os.Setenv(tc.secret, "secret")
			e := EnvProvider{}
			act, _ := e.Retrieve(context.TODO())
			exp := aws.Credentials{AccessKeyID: "key", SecretAccessKey: "secret", SessionToken: ""}
			assert.Equal(t, exp, act)
		})
	}
}
