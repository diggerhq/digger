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
  - resource "docker_image" "aaa" {

    }


Unless you have made equivalent changes to your digger_config, or ignored the
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

func TestCorrectCleanUpWithOpenTofuPlan(t *testing.T) {
	stdout := `
Initializing the backend...
Initializing modules...
Initializing provider plugins...
- Reusing previous version of hashicorp/helm from the dependency lock file
- Reusing previous version of hashicorp/google-beta from the dependency lock file
- Reusing previous version of ns1-terraform/ns1 from the dependency lock file
- Reusing previous version of hashicorp/google from the dependency lock file
- Reusing previous version of hashicorp/time from the dependency lock file
- Reusing previous version of hashicorp/random from the dependency lock file
- Reusing previous version of integrations/github from the dependency lock file
- Reusing previous version of hashicorp/kubernetes from the dependency lock file
- Reusing previous version of hashicorp/tls from the dependency lock file
- Reusing previous version of hashicorp/vault from the dependency lock file
- Using previously-installed hashicorp/helm v2.10.1
- Using previously-installed ns1-terraform/ns1 v2.0.3
- Using previously-installed hashicorp/google v5.11.0
- Using previously-installed hashicorp/time v0.9.1
- Using previously-installed hashicorp/random v3.5.1
- Using previously-installed hashicorp/vault v3.15.2
- Using previously-installed hashicorp/google-beta v5.11.0
- Using previously-installed integrations/github v5.26.0
- Using previously-installed hashicorp/kubernetes v2.21.0
- Using previously-installed hashicorp/tls v4.0.4
OpenTofu has been successfully initialized!
data.vault_generic_secret.ns1_api_key_terraform: Reading...
data.vault_generic_secret.github_token: Reading...
data.vault_generic_secret.ns1_api_key_terraform: Read complete after 0s [id=zon/v1/ns1/api/terraform]
data.vault_generic_secret.github_token: Read complete after 0s [id=zon/v1/github/terraform/api-token]
data.google_project.project: Reading...
data.google_client_config.this: Reading...
module.infra_data.data.google_client_config.this: Reading...
module.infra_data.data.google_project.this: Reading...
data.google_client_config.this: Read complete after 1s [id=projects/"xxx"/regions/"europe-west3"/zones/"europe-west3-a"]
module.infra_data.data.google_client_config.this: Read complete after 1s [id=projects/"xxx"/regions/"europe-west3"/zones/"europe-west3-a"]
data.google_project.project: Read complete after 1s [id=projects/xxx]
module.infra_data.data.google_project.this: Read complete after 2s [id=projects/xxx]
module.infra_data.data.vault_kv_secrets_list.cloudsql_instances[0]: Reading...
module.infra_data.data.google_project.gke: Reading...
module.infra_data.data.vault_kv_secrets_list.cloudsql_instances[0]: Read complete after 0s [id=zon/v1/gcp/xxx/cloudsql/instances]
module.infra_data.data.google_sql_database_instance.this["xxx"]: Reading...
module.infra_data.data.google_sql_database_instance.this["xxx"]: Reading...
module.infra_data.data.google_sql_database_instance.this["xxx"]: Reading...
module.infra_data.data.google_sql_database_instance.this["xxx"]: Reading...
module.infra_data.data.google_sql_database_instance.this["xxx"]: Reading...
module.infra_data.data.google_sql_database_instance.this["xxx"]: Reading...
module.infra_data.data.google_sql_database_instance.this["xxx"]: Reading...
module.infra_data.data.google_sql_database_instance.this["xxx"]: Reading...
module.infra_data.data.google_sql_database_instance.this["xxx"]: Reading...
module.infra_data.data.google_sql_database_instance.this["xxx"]: Read complete after 0s [id=main-staging-pg14]
module.infra_data.data.google_sql_database_instance.this["xxx"]: Reading...
module.infra_data.data.google_sql_database_instance.this["xxx"]: Read complete after 0s [id=main-production-vivi-internal]
module.infra_data.data.google_sql_database_instance.this["xxx"]: Read complete after 0s [id=staging-9f9d9107]
module.infra_data.data.google_sql_database_instance.this["xxx"]: Read complete after 0s [id=devel-26e8a66a]
module.infra_data.data.google_sql_database_instance.this["xxx"]: Read complete after 0s [id=production-a31270fe]
module.infra_data.data.google_sql_database_instance.this["xxx"]: Reading...
module.infra_data.data.google_sql_database_instance.this["xxx"]: Reading...
module.infra_data.data.google_sql_database_instance.this["xxx"]: Read complete after 0s [id=main-production-pg14-rr]
module.infra_data.data.google_sql_database_instance.this["xxx"]: Read complete after 0s [id=production-pg13]
module.infra_data.data.google_sql_database_instance.this["xxx"]: Read complete after 0s [id=main-staging-pg15]
module.infra_data.data.google_sql_database_instance.this["xxx"]: Read complete after 0s [id=main-production-pg14]
module.infra_data.data.google_sql_database_instance.this["xxx"]: Read complete after 0s [id=main-devel-pg15]
module.infra_data.data.google_sql_database_instance.this["xxx"]: Read complete after 0s [id=main-devel-pg14]
module.infra_data.data.google_sql_database_instance.this["xxx"]: Read complete after 0s [id=staging-pg13]
module.infra_data.data.vault_generic_secret.cloudsql_credentials["xxx"]: Reading...
module.infra_data.data.vault_generic_secret.cloudsql_credentials["xxx"]: Reading...
module.infra_data.data.vault_generic_secret.cloudsql_credentials["xxx"]: Reading...
module.infra_data.data.vault_generic_secret.cloudsql_credentials["xxx"]: Reading...
module.infra_data.data.vault_generic_secret.cloudsql_credentials["xxx"]: Reading...
module.infra_data.data.vault_generic_secret.cloudsql_credentials["xxx"]: Reading...
module.infra_data.data.vault_generic_secret.cloudsql_credentials["xxx"]: Reading...
module.infra_data.data.vault_generic_secret.cloudsql_credentials["xxx"]: Read complete after 0s [id=zon/v1/gcp/xxx/cloudsql/instances/staging-pg13/instance-credentials]
module.infra_data.data.vault_generic_secret.cloudsql_credentials["xxx"]: Reading...
module.infra_data.data.vault_generic_secret.cloudsql_credentials["xxx"]: Read complete after 0s [id=zon/v1/gcp/xxx/cloudsql/instances/main-devel-pg15/instance-credentials]
module.infra_data.data.vault_generic_secret.cloudsql_credentials["xxx"]: Reading...
module.infra_data.data.vault_generic_secret.cloudsql_credentials["xxx"]: Read complete after 0s [id=zon/v1/gcp/xxx/cloudsql/instances/main-production-pg14-rr/instance-credentials]
module.infra_data.data.vault_generic_secret.cloudsql_credentials["xxx"]: Read complete after 0s [id=zon/v1/gcp/xxx/cloudsql/instances/staging-9f9d9107/instance-credentials]
module.infra_data.data.vault_generic_secret.cloudsql_credentials["xxx"]: Read complete after 0s [id=zon/v1/gcp/xxx/cloudsql/instances/devel-26e8a66a/instance-credentials]
module.infra_data.data.vault_generic_secret.cloudsql_credentials["xxx"]: Read complete after 0s [id=zon/v1/gcp/xxx/cloudsql/instances/main-devel-pg14/instance-credentials]
module.infra_data.data.vault_generic_secret.cloudsql_credentials["xxx"]: Read complete after 0s [id=zon/v1/gcp/xxx/cloudsql/instances/main-production-pg14/instance-credentials]
module.infra_data.data.vault_generic_secret.cloudsql_credentials["xxx"]: Read complete after 0s [id=zon/v1/gcp/xxx/cloudsql/instances/main-staging-pg15/instance-credentials]
module.infra_data.data.vault_generic_secret.cloudsql_credentials["xxx"]: Read complete after 0s [id=zon/v1/gcp/xxx/cloudsql/instances/production-a31270fe/instance-credentials]
module.infra_data.data.vault_generic_secret.cloudsql_credentials["xxx"]: Reading...
module.infra_data.data.vault_generic_secret.cloudsql_credentials["xxx"]: Reading...
module.infra_data.data.vault_generic_secret.cloudsql_credentials["xxx"]: Reading...
module.infra_data.data.vault_generic_secret.cloudsql_credentials["xxx"]: Read complete after 0s [id=zon/v1/gcp/xxx/cloudsql/instances/main-production-vivi-internal/instance-credentials]
module.infra_data.data.vault_generic_secret.cloudsql_credentials["xxx"]: Read complete after 0s [id=zon/v1/gcp/xxx/cloudsql/instances/production-pg13/instance-credentials]
module.infra_data.data.vault_generic_secret.cloudsql_credentials["xxx"]: Read complete after 0s [id=zon/v1/gcp/xxx/cloudsql/instances/main-staging-pg14/instance-credentials]
module.infra_data.data.google_project.gke: Read complete after 0s [id=projects/xxx]
module.infra_data.data.vault_generic_secret.cluster_credentials[0]: Reading...
module.infra_data.data.vault_generic_secret.cluster_credentials[0]: Read complete after 0s [id=zon/v1/gcp/xxx/gke/staging/cluster_credentials]
OpenTofu used the selected providers to generate the following execution
plan. Resource actions are indicated with the following symbols:
  + create
OpenTofu will perform the following actions:
  # google_storage_bucket.test1 will be created
  + resource "google_storage_bucket" "test1" {
      + effective_labels            = {
          + "creator" = "terraform"
        }
      + force_destroy               = false
      + id                          = (known after apply)
      + labels                      = {
          + "creator" = "terraform"
        }
      + location                    = "EUROPE-WEST3"
      + name                        = "digger-poc-staging-test1"
      + project                     = (known after apply)
      + public_access_prevention    = (known after apply)
      + rpo                         = (known after apply)
      + self_link                   = (known after apply)
      + storage_class               = "STANDARD"
      + terraform_labels            = {
          + "creator" = "terraform"
        }
      + uniform_bucket_level_access = true
      + url                         = (known after apply)
    }
Plan: 1 to add, 0 to change, 0 to destroy.
`
	res := cleanupTerraformPlan(true, nil, stdout, "")
	index := strings.Index(stdout, "OpenTofu will perform the following actions:")
	assert.Equal(t, stdout[index:], res)
}
