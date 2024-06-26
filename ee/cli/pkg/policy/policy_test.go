package policy

import (
	"github.com/stretchr/testify/assert"
	"log"
	"os"
	"testing"
)

func init() {
	log.SetOutput(os.Stdout)
	log.SetFlags(log.Ldate | log.Ltime)
}

func TestGetPrefixesForPath(t *testing.T) {
	prefixes := GetPrefixesForPath("dev/vpc/subnets", "access.rego")
	assert.Equal(t, prefixes, []string{"dev/vpc/subnets/access.rego", "dev/vpc/access.rego", "dev/access.rego"})
	log.Printf("%v", prefixes)
}
