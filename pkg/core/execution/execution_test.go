package execution

import (
	"github.com/stretchr/testify/assert"
	"strings"
	"testing"
)

func TestCorrectCleanUpWithoutRegexDoesNotProduceException(t *testing.T) {
	stdout := `
Note: Objects have changed outside of Terraform

Terraform detected the following changes made outside of Terraform since the
last "terraform apply" which may have affected this plan:

  # docker_image.xxxx has been deleted
  - resource "docker_image" "ac2ical" {

    }


Unless you have made equivalent changes to your configuration, or ignored the
relevant attributes using ignore_changes, the following plan may include
actions to undo or respond to these changes.

─────────────────────────────────────────────────────────────────────────────

Terraform used the selected providers to generate the following execution
plan. Resource actions are indicated with the following symbols:
  + create

Terraform will perform the following actions:

  # docker_image.xxxx will be created
  + resource "docker_image" "xxx" {
      + id          = (known after apply)
      + image_id    = (known after apply)
      + name        = "***.dkr.ecr.***.amazonaws.com/xxx-yyy:latest"
      + repo_digest = (known after apply)
      + triggers    = {
          + "dir_sha1" = "aaaa"
        }

      + build {
          + cache_from   = []
          + context      = "docker/"
          + dockerfile   = "Dockerfile"
          + extra_hosts  = []
          + remove       = true
          + security_opt = []
          + tag          = [
              + "xxx:latest",
            ]
        }
    }

  # docker_registry_image.xxx will be created
  + resource "docker_registry_image" "xxx" {
      + id                   = (known after apply)
      + insecure_skip_verify = false
      + keep_remotely        = true
      + name                 = "***.dkr.ecr.***.amazonaws.com/xxx-yyy:latest"
      + sha256_digest        = (known after apply)
    }

Plan: 2 to add, 0 to change, 0 to destroy.
`
	res := cleanupTerraformPlan(true, nil, stdout, "")
	index := strings.Index(stdout, "Terraform will perform the following actions:")
	assert.Equal(t, stdout[index:], res)
}
