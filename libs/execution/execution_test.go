package execution

import (
	"github.com/stretchr/testify/assert"
	"log"
	"os"
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
data.archive_file.code: Reading...
data.archive_file.code: Read complete after 0s [id=b820080f55920896a4fea9e94200cd0287f05854]

OpenTofu used the selected providers to generate the following execution plan. Resource actions are indicated with the following symbols:
  + create
 <= read (data resources)

OpenTofu will perform the following actions:

  # data.archive_file.layer will be read during apply
  # (depends on a resource or a module with changes pending)
 <= data "archive_file" "layer" {
      + id                  = (known after apply)
      + output_base64sha256 = (known after apply)
      + output_base64sha512 = (known after apply)
      + output_md5          = (known after apply)
      + output_path         = "./layer.zip"
      + output_sha          = (known after apply)
      + output_sha256       = (known after apply)
      + output_sha512       = (known after apply)
      + output_size         = (known after apply)
      + source_dir          = "./layer"
      + type                = "zip"
    }

  # aws_apigatewayv2_api.api will be created
  + resource "aws_apigatewayv2_api" "api" {
      + api_endpoint                 = (known after apply)
      + api_key_selection_expression = "$request.header.x-api-key"
      + arn                          = (known after apply)
      + execution_arn                = (known after apply)
      + id                           = (known after apply)
      + name                         = "test-api"
      + protocol_type                = "HTTP"
      + route_selection_expression   = "$request.method $request.path"
      + tags_all                     = (known after apply)
    }

  # aws_apigatewayv2_integration.integration will be created
  + resource "aws_apigatewayv2_integration" "integration" {
      + api_id                                    = (known after apply)
      + connection_type                           = "INTERNET"
      + id                                        = (known after apply)
      + integration_response_selection_expression = (known after apply)
      + integration_type                          = "AWS_PROXY"
      + integration_uri                           = (known after apply)
      + payload_format_version                    = "2.0"
      + timeout_milliseconds                      = (known after apply)
    }

  # aws_apigatewayv2_route.route will be created
  + resource "aws_apigatewayv2_route" "route" {
      + api_id             = (known after apply)
      + api_key_required   = false
      + authorization_type = "NONE"
      + id                 = (known after apply)
      + route_key          = "ANY /test-api-lambda"
      + target             = (known after apply)
    }

  # aws_apigatewayv2_stage.stage will be created
  + resource "aws_apigatewayv2_stage" "stage" {
      + api_id        = (known after apply)
      + arn           = (known after apply)
      + auto_deploy   = true
      + deployment_id = (known after apply)
      + execution_arn = (known after apply)
      + id            = (known after apply)
      + invoke_url    = (known after apply)
      + name          = "test-stage"
      + tags_all      = (known after apply)
    }

  # aws_iam_role.iam_role will be created
  + resource "aws_iam_role" "iam_role" {
      + arn                   = (known after apply)
      + assume_role_policy    = jsonencode(
            {
              + Statement = [
                  + {
                      + Action    = "sts:AssumeRole"
                      + Effect    = "Allow"
                      + Principal = {
                          + Service = "lambda.amazonaws.com"
                        }
                      + Sid       = ""
                    },
                ]
              + Version   = "2012-10-17"
            }
        )
      + create_date           = (known after apply)
      + force_detach_policies = false
      + id                    = (known after apply)
      + managed_policy_arns   = (known after apply)
      + max_session_duration  = 3600
      + name                  = "api-lambda-iam-role"
      + name_prefix           = (known after apply)
      + path                  = "/"
      + tags_all              = (known after apply)
      + unique_id             = (known after apply)
    }

  # aws_lambda_function.lambda will be created
  + resource "aws_lambda_function" "lambda" {
      + architectures                  = (known after apply)
      + arn                            = (known after apply)
      + filename                       = "./code.zip"
      + function_name                  = "test-api-lambda"
      + handler                        = "lambda.main"
      + id                             = (known after apply)
      + invoke_arn                     = (known after apply)
      + last_modified                  = (known after apply)
      + layers                         = (known after apply)
      + memory_size                    = 128
      + package_type                   = "Zip"
      + publish                        = false
      + qualified_arn                  = (known after apply)
      + qualified_invoke_arn           = (known after apply)
      + reserved_concurrent_executions = -1
      + role                           = (known after apply)
      + runtime                        = "python3.9"
      + signing_job_arn                = (known after apply)
      + signing_profile_version_arn    = (known after apply)
      + skip_destroy                   = false
      + source_code_hash               = "ublI6yckpbU+C/XOPeXreDe9BMtYmC5NmHtpxdFa0E0="
      + source_code_size               = (known after apply)
      + tags_all                       = (known after apply)
      + timeout                        = 3
      + version                        = (known after apply)

      + environment {
          + variables = {
              + "MESSAGE" = "Terraform sends its regards"
            }
        }
    }

  # aws_lambda_layer_version.layer will be created
  + resource "aws_lambda_layer_version" "layer" {
      + arn                         = (known after apply)
      + compatible_runtimes         = [
          + "python3.6",
          + "python3.7",
          + "python3.8",
          + "python3.9",
        ]
      + created_date                = (known after apply)
      + filename                    = "./layer.zip"
      + id                          = (known after apply)
      + layer_arn                   = (known after apply)
      + layer_name                  = "test-api-layer"
      + signing_job_arn             = (known after apply)
      + signing_profile_version_arn = (known after apply)
      + skip_destroy                = false
      + source_code_hash            = (known after apply)
      + source_code_size            = (known after apply)
      + version                     = (known after apply)
    }

  # aws_lambda_permission.api will be created
  + resource "aws_lambda_permission" "api" {
      + action              = "lambda:InvokeFunction"
      + function_name       = (known after apply)
      + id                  = (known after apply)
      + principal           = "apigateway.amazonaws.com"
      + source_arn          = (known after apply)
      + statement_id        = "AllowExecutionFromAPIGateway"
      + statement_id_prefix = (known after apply)
    }

  # null_resource.pip_install will be created
  + resource "null_resource" "pip_install" {
      + id       = (known after apply)
      + triggers = {
          + "shell_hash" = "e29a8fbd0127f5ea361fb998fc6a150bec13d9d834f2dad4fb4e6a5d8497bb71"
        }
    }

Plan: 9 to add, 0 to change, 0 to destroy.

Changes to Outputs:
  + api_url = (known after apply)
`
	res := cleanupTerraformPlan(true, nil, stdout, "")
	index := strings.Index(stdout, "OpenTofu will perform the following actions:")
	assert.Equal(t, stdout[index:], res)
}

func TestCleanupTrg(t *testing.T) {
	stdout := `23:21:39.514 STDOUT terraform: aws_s3_bucket.example: Refreshing state... [id=my-tf-test-bucket20240516184950890200000001]
23:21:39.757 STDOUT terraform: Terraform used the selected providers to generate the following execution
23:21:39.757 STDOUT terraform: plan. Resource actions are indicated with the following symbols:
23:21:39.757 STDOUT terraform:   ~ update in-place
23:21:39.757 STDOUT terraform: Terraform will perform the following actions:
23:21:39.757 STDOUT terraform:   # aws_s3_bucket.example will be updated in-place
23:21:39.757 STDOUT terraform:   ~ resource "aws_s3_bucket" "example" {
23:21:39.757 STDOUT terraform:         id                          = "my-tf-test-bucket20240516184950890200000001"
23:21:39.757 STDOUT terraform:       ~ tags                        = {
23:21:39.757 STDOUT terraform:             "Environment" = "env5"
23:21:39.757 STDOUT terraform:           ~ "Name"        = "My bucket env534" -> "My bucket env535"
23:21:39.757 STDOUT terraform:         }
23:21:39.757 STDOUT terraform:       ~ tags_all                    = {
23:21:39.757 STDOUT terraform:           ~ "Name"        = "My bucket env534" -> "My bucket env535"
23:21:39.758 STDOUT terraform:             # (1 unchanged element hidden)
23:21:39.758 STDOUT terraform:         }
23:21:39.758 STDOUT terraform:         # (12 unchanged attributes hidden)
23:21:39.758 STDOUT terraform:         # (3 unchanged blocks hidden)
23:21:39.758 STDOUT terraform:     }
23:21:39.758 STDOUT terraform: Plan: 0 to add, 1 to change, 0 to destroy.`
	res := cleanupTerraformPlan(true, nil, stdout, "")
	log.Printf("the result is %v", res)
}

func init() {
	log.SetOutput(os.Stdout)
	log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)
	log.Println("Initialized the logger successfully")
}
