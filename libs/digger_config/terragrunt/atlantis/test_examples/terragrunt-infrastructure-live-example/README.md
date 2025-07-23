[![Maintained by Gruntwork.io](https://img.shields.io/badge/maintained%20by-gruntwork.io-%235849a6.svg)](https://gruntwork.io/?ref=repo_terragrunt-infra-live-example)

# Example infrastructure-live for Terragrunt

This repo, along with the [terragrunt-infrastructure-modules-example
repo](https://github.com/gruntwork-io/terragrunt-infrastructure-modules-example), show an example file/folder structure
you can use with [Terragrunt](https://github.com/gruntwork-io/terragrunt) to keep your
[Terraform](https://www.terraform.io) code DRY. For background information, check out the [Keep your Terraform code
DRY](https://github.com/gruntwork-io/terragrunt#keep-your-terraform-code-dry) section of the Terragrunt documentation.

This repo shows an example of how to use the modules from the `terragrunt-infrastructure-modules-example` repo to
deploy an Auto Scaling Group (ASG) and a MySQL DB across three environments (qa, stage, prod) and two AWS accounts
(non-prod, prod), all without duplicating any of the Terraform code. That's because there is just a single copy of
the Terraform code, defined in the `terragrunt-infrastructure-modules-example` repo, and in this repo, we solely define
`terragrunt.hcl` files that reference that code (at a specific version, too!) and fill in variables specific to each
environment.

Be sure to read through [the Terragrunt documentation on DRY
Architectures](https://terragrunt.gruntwork.io/docs/features/keep-your-terragrunt-architecture-dry/) to understand the
features of Terragrunt used in this folder organization.

Note: This code is solely for demonstration purposes. This is not production-ready code, so use at your own risk. If
you are interested in battle-tested, production-ready Terraform code, check out [Gruntwork](http://www.gruntwork.io/).




## How do you deploy the infrastructure in this repo?


### Pre-requisites

1. Install [Terraform](https://www.terraform.io/) version `0.13.0` or newer and
   [Terragrunt](https://github.com/gruntwork-io/terragrunt) version `v0.32.0` or newer.
1. Update the `bucket` parameter in the root `terragrunt.hcl`. We use S3 [as a Terraform
   backend](https://www.terraform.io/docs/backends/types/s3.html) to store your
   Terraform state, and S3 bucket names must be globally unique. The name currently in
   the file is already taken, so you'll have to specify your own. Alternatives, you can
   set the environment variable `TG_BUCKET_PREFIX` to set a custom prefix.
1. Configure your AWS credentials using one of the supported [authentication
   mechanisms](https://www.terraform.io/docs/providers/aws/#authentication).
1. Fill in your AWS Account ID's in `prod/account.hcl` and `non-prod/account.hcl`.


### Deploying a single module

1. `cd` into the module's folder (e.g. `cd non-prod/us-east-1/qa/mysql`).
1. Note: if you're deploying the MySQL DB, you'll need to configure your DB password as an environment variable:
   `export TF_VAR_master_password=(...)`.
1. Run `terragrunt plan` to see the changes you're about to apply.
1. If the plan looks good, run `terragrunt apply`.


### Deploying all modules in a region

1. `cd` into the region folder (e.g. `cd non-prod/us-east-1`).
1. Configure the password for the MySQL DB as an environment variable: `export TF_VAR_master_password=(...)`.
1. Run `terragrunt plan-all` to see all the changes you're about to apply.
1. If the plan looks good, run `terragrunt apply-all`.


### Testing the infrastructure after it's deployed

After each module is finished deploying, it will write a bunch of outputs to the screen. For example, the ASG will
output something like the following:

```
Outputs:

asg_name = tf-asg-00343cdb2415e9d5f20cda6620
asg_security_group_id = sg-d27df1a3
elb_dns_name = webserver-example-prod-1234567890.us-east-1.elb.amazonaws.com
elb_security_group_id = sg-fe62ee8f
url = http://webserver-example-prod-1234567890.us-east-1.elb.amazonaws.com:80
```

A minute or two after the deployment finishes, and the servers in the ASG have passed their health checks, you should
be able to test the `url` output in your browser or with `curl`:

```
curl http://webserver-example-prod-1234567890.us-east-1.elb.amazonaws.com:80

Hello, World
```

Similarly, the MySQL module produces outputs that will look something like this:

```
Outputs:

arn = arn:aws:rds:us-east-1:1234567890:db:terraform-00d7a11c1e02cf617f80bbe301
db_name = mysql_prod
endpoint = terraform-1234567890.abcdefghijklmonp.us-east-1.rds.amazonaws.com:3306
```

You can use the `endpoint` and `db_name` outputs with any MySQL client:

```
mysql --host=terraform-1234567890.abcdefghijklmonp.us-east-1.rds.amazonaws.com:3306 --user=admin --password mysql_prod
```






## How is the code in this repo organized?

The code in this repo uses the following folder hierarchy:

```
account
 └ _global
 └ region
    └ _global
    └ environment
       └ resource
```

Where:

* **Account**: At the top level are each of your AWS accounts, such as `stage-account`, `prod-account`, `mgmt-account`,
  etc. If you have everything deployed in a single AWS account, there will just be a single folder at the root (e.g.
  `main-account`).

* **Region**: Within each account, there will be one or more [AWS
  regions](http://docs.aws.amazon.com/AWSEC2/latest/UserGuide/using-regions-availability-zones.html), such as
  `us-east-1`, `eu-west-1`, and `ap-southeast-2`, where you've deployed resources. There may also be a `_global`
  folder that defines resources that are available across all the AWS regions in this account, such as IAM users,
  Route 53 hosted zones, and CloudTrail.

* **Environment**: Within each region, there will be one or more "environments", such as `qa`, `stage`, etc. Typically,
  an environment will correspond to a single [AWS Virtual Private Cloud (VPC)](https://aws.amazon.com/vpc/), which
  isolates that environment from everything else in that AWS account. There may also be a `_global` folder
  that defines resources that are available across all the environments in this AWS region, such as Route 53 A records,
  SNS topics, and ECR repos.

* **Resource**: Within each environment, you deploy all the resources for that environment, such as EC2 Instances, Auto
  Scaling Groups, ECS Clusters, Databases, Load Balancers, and so on. Note that the Terraform code for most of these
  resources lives in the [terragrunt-infrastructure-modules-example repo](https://github.com/gruntwork-io/terragrunt-infrastructure-modules-example).

## Creating and using root (account) level variables

In the situation where you have multiple AWS accounts or regions, you often have to pass common variables down to each
of your modules. Rather than copy/pasting the same variables into each `terragrunt.hcl` file, in every region, and in
every environment, you can inherit them from the `inputs` defined in the root `terragrunt.hcl` file.
