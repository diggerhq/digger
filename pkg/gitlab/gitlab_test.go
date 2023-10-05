package gitlab

import (
	configuration "github.com/diggerhq/lib-digger-config"
	"github.com/joho/godotenv"
	"github.com/stretchr/testify/assert"
	"os"
	"strings"
	"testing"
)

var newMergeRequestEnvVars = `
CI_PROJECT_NAMESPACE=diggerdev
GITLAB_USER_ID=13159253
CI_RUNNER_VERSION=16.3.0~beta.108.g2b6048b4
CI_MERGE_REQUEST_TARGET_BRANCH_PROTECTED=true
FF_SKIP_NOOP_BUILD_STAGES=true
CI_SERVER_NAME=GitLab
CI_RUNNER_DESCRIPTION=4-green.saas-linux-small-amd64.runners-manager.gitlab.com/default
GITLAB_USER_EMAIL=alexey@digger.dev
CI_SERVER_REVISION=176e0aff712
CI_MERGE_REQUEST_SOURCE_BRANCH_PROTECTED=false
FF_USE_WINDOWS_LEGACY_PROCESS_STRATEGY=true
CI_MERGE_REQUEST_SOURCE_BRANCH_NAME=test-dev
CI_MERGE_REQUEST_TARGET_BRANCH_SHA=ffaf8ce1eb166a318fbb43e25ce708f37675d174
CI_RUNNER_EXECUTABLE_ARCH=linux/amd64
CI_PIPELINE_NAME=
CI_REGISTRY_USER=gitlab-ci-token
CI_API_V4_URL=https://gitlab.com/api/v4
CI_REGISTRY_PASSWORD=[MASKED]
CI_RUNNER_SHORT_TOKEN=ntHFEtyX
CI_JOB_NAME=print_env
CI_OPEN_MERGE_REQUESTS=diggerdev/digger-demo!45
HOSTNAME=runner-nthfetyx-project-44723537-concurrent-0
GITLAB_USER_LOGIN=alexey_digger
CI_PROJECT_NAME=digger-demo
CI_PIPELINE_SOURCE=merge_request_event
FF_RETRIEVE_POD_WARNING_EVENTS=false
CI_JOB_STATUS=running
CI_PIPELINE_ID=1026565167
FF_DISABLE_POWERSHELL_STDIN=false
CI_COMMIT_REF_SLUG=test-dev
CI_MERGE_REQUEST_SOURCE_PROJECT_PATH=diggerdev/digger-demo
CI_SERVER=yes
FF_SET_PERMISSIONS_BEFORE_CLEANUP=true
CI_COMMIT_SHORT_SHA=b8c339f0
CI_JOB_NAME_SLUG=print-env
RUNNER_TEMP_PROJECT_DIR=/builds/diggerdev/digger-demo.tmp
FF_CMD_DISABLE_DELAYED_ERROR_LEVEL_EXPANSION=false
CI_DEPENDENCY_PROXY_GROUP_IMAGE_PREFIX=gitlab.com:443/diggerdev/dependency_proxy/containers
AWS_REGION=us-east-1
PWD=/builds/diggerdev/digger-demo
CI_RUNNER_TAGS=["gce", "east-c", "linux", "ruby", "mysql", "postgres", "mongo", "git-annex", "shared", "docker", "saas-linux-small-amd64"]
CI_MERGE_REQUEST_DIFF_BASE_SHA=ffaf8ce1eb166a318fbb43e25ce708f37675d174
CI_PROJECT_PATH=diggerdev/digger-demo
CI_MERGE_REQUEST_SOURCE_PROJECT_URL=https://gitlab.com/diggerdev/digger-demo
FF_USE_NEW_BASH_EVAL_STRATEGY=false
CI_SERVER_TLS_CA_FILE=/builds/diggerdev/digger-demo.tmp/CI_SERVER_TLS_CA_FILE
CI_DEPENDENCY_PROXY_DIRECT_GROUP_IMAGE_PREFIX=gitlab.com:443/diggerdev/dependency_proxy/containers
CI_MERGE_REQUEST_PROJECT_URL=https://gitlab.com/diggerdev/digger-demo
CI_COMMIT_REF_PROTECTED=false
FF_USE_POWERSHELL_PATH_RESOLVER=false
CI_MERGE_REQUEST_TITLE=Update main.tf
CI_API_GRAPHQL_URL=https://gitlab.com/api/graphql
CI_SERVER_VERSION_MINOR=5
CI_COMMIT_SHA=b8c339f048d4d1296ca5791584158744b166df83
HOME=/root
FF_NETWORK_PER_BUILD=false
CI_DEPENDENCY_PROXY_PASSWORD=[MASKED]
CI_JOB_TIMEOUT=3600
CI_PROJECT_VISIBILITY=private
CI_CONCURRENT_PROJECT_ID=0
FF_SCRIPT_SECTIONS=false
CI_COMMIT_MESSAGE="Merge branch 'test-dev' into 'main'\n
Update main.tf\n
See merge request diggerdev/digger-demo!45"
DOCKER_TLS_CERTDIR=
CI_SERVER_SHELL_SSH_PORT=22
CI_JOB_JWT_V1=[MASKED]
CI_JOB_JWT_V2=[MASKED]
FF_USE_DIRECT_DOWNLOAD=true
CI_PAGES_DOMAIN=gitlab.io
CI_SERVER_VERSION=16.5.0-pre
CI_MERGE_REQUEST_PROJECT_PATH=diggerdev/digger-demo
FF_USE_POD_ACTIVE_DEADLINE_SECONDS=false
CI_REGISTRY=registry.gitlab.com
CI_SERVER_PORT=443
CI_MERGE_REQUEST_IID=45
AWS_SECRET_ACCESS_KEY=[MASKED]
CI_PROJECT_NAMESPACE_ID=12854814
GOLANG_VERSION=1.20.8
FF_USE_IMPROVED_URL_MASKING=false
CI_MERGE_REQUEST_PROJECT_ID=44723537
CI_MERGE_REQUEST_ID=255037359
CI_PAGES_URL=https://diggerdev.gitlab.io/digger-demo
CI_PIPELINE_IID=524
CI_REPOSITORY_URL=https://gitlab-ci-token:[MASKED]@gitlab.com/diggerdev/digger-demo.git
CI_SERVER_URL=https://gitlab.com
FF_ENABLE_BASH_EXIT_CODE_CHECK=false
GITLAB_FEATURES=audit_events,blocked_issues,board_iteration_lists,code_owners,code_review_analytics,contribution_analytics,elastic_search,full_codequality_report,group_activity_analytics,group_bulk_edit,group_webhooks,issuable_default_templates,issue_weights,iterations,ldap_group_sync,member_lock,merge_request_approvers,milestone_charts,multiple_issue_assignees,multiple_ldap_servers,multiple_merge_request_assignees,multiple_merge_request_reviewers,project_merge_request_analytics,protected_refs_for_users,push_rules,repository_mirrors,resource_access_token,seat_link,usage_quotas,visual_review_app,wip_limits,zoekt_code_search,blocked_work_items,description_diffs,send_emails_from_admin_area,repository_size_limit,maintenance_mode,scoped_issue_board,adjourned_deletion_for_projects_and_groups,admin_audit_log,auditor_user,blocking_merge_requests,board_assignee_lists,board_milestone_lists,ci_cd_projects,ci_namespace_catalog,ci_secrets_management,cluster_agents_ci_impersonation,cluster_agents_user_impersonation,cluster_deployments,code_owner_approval_required,code_suggestions,commit_committer_check,commit_committer_name_check,compliance_framework,custom_compliance_frameworks,cross_project_pipelines,custom_file_templates,custom_file_templates_for_namespace,custom_project_templates,cycle_analytics_for_groups,cycle_analytics_for_projects,db_load_balancing,default_branch_protection_restriction_in_groups,default_project_deletion_protection,delete_unconfirmed_users,dependency_proxy_for_packages,disable_name_update_for_users,disable_personal_access_tokens,domain_verification,email_additional_text,epics,extended_audit_events,external_authorization_service_api_management,feature_flags_related_issues,feature_flags_code_references,file_locks,geo,generic_alert_fingerprinting,git_two_factor_enforcement,github_integration,group_allowed_email_domains,group_coverage_reports,group_forking_protection,group_milestone_project_releases,group_project_templates,group_repository_analytics,group_saml,group_scoped_ci_variables,group_wikis,ide_schema_config,incident_metric_upload,incident_sla,instance_level_scim,issues_analytics,jira_issues_integration,ldap_group_sync_filter,merge_pipelines,merge_request_performance_metrics,admin_merge_request_approvers_rules,merge_trains,metrics_reports,multiple_alert_http_integrations,multiple_approval_rules,multiple_group_issue_boards,object_storage,microsoft_group_sync,operations_dashboard,package_forwarding,pages_size_limit,pages_multiple_versions,productivity_analytics,project_aliases,protected_environments,reject_non_dco_commits,reject_unsigned_commits,remote_development,saml_group_sync,service_accounts,scoped_labels,smartcard_auth,ssh_certificates,swimlanes,target_branch_rules,type_of_work_analytics,minimal_access_role,unprotection_restrictions,ci_project_subscriptions,incident_timeline_view,oncall_schedules,escalation_policies,export_user_permissions,zentao_issues_integration,coverage_check_approval_rule,issuable_resource_links,group_protected_branches,group_level_merge_checks_setting,oidc_client_groups_claim,disable_deleting_account_for_users,group_ip_restriction,password_complexity,enterprise_templates,git_abuse_rate_limit,required_ci_templates,runner_maintenance_note,runner_performance_insights,runner_upgrade_management
CI_MERGE_REQUEST_REF_PATH=refs/merge-requests/45/head
CI_COMMIT_DESCRIPTION="\n
Update main.tf\n
See merge request diggerdev/digger-demo!45"
FF_USE_ADVANCED_POD_SPEC_CONFIGURATION=false
CI_TEMPLATE_REGISTRY_HOST=registry.gitlab.com
CI_JOB_STAGE=digger
CI_MERGE_REQUEST_DIFF_ID=807628623
CI_PIPELINE_URL=https://gitlab.com/diggerdev/digger-demo/-/pipelines/1026565167
CI_DEFAULT_BRANCH=main
CI_MERGE_REQUEST_TARGET_BRANCH_NAME=main
CI_MERGE_REQUEST_SOURCE_BRANCH_SHA=a926daf13cb8f96f2e5d6c9e09bb28daf7763309
CI_MERGE_REQUEST_SQUASH_ON_MERGE=false
CI_SERVER_VERSION_PATCH=0
CI_COMMIT_TITLE=Merge branch 'test-dev' into 'main'
CI_PROJECT_ROOT_NAMESPACE=diggerdev
FF_ENABLE_JOB_CLEANUP=false
FF_RESOLVE_FULL_TLS_CHAIN=true
GITLAB_USER_NAME=Alexey Skriptsov
CI_MERGE_REQUEST_SOURCE_PROJECT_ID=44723537
CI_PROJECT_DIR=/builds/diggerdev/digger-demo
CI_MERGE_REQUEST_EVENT_TYPE=merged_result
SHLVL=1
CI_RUNNER_ID=12270857
CI_PIPELINE_CREATED_AT=2023-10-05T09:51:02Z
CI_COMMIT_TIMESTAMP=2023-10-05T09:51:01+00:00
AWS_ACCESS_KEY_ID=AKIATBSMGQ3XRUHRM725
CI_DISPOSABLE_ENVIRONMENT=true
CI_SERVER_SHELL_SSH_HOST=gitlab.com
CI_JOB_JWT=[MASKED]
CI_REGISTRY_IMAGE=registry.gitlab.com/diggerdev/digger-demo
CI_SERVER_PROTOCOL=https
CI_MERGE_REQUEST_APPROVED=true
CI_COMMIT_AUTHOR=Alexey Skriptsov <alexey@digger.dev>
FF_POSIXLY_CORRECT_ESCAPES=false
CI_COMMIT_REF_NAME=test-dev
CI_SERVER_HOST=gitlab.com
CI_JOB_URL=https://gitlab.com/diggerdev/digger-demo/-/jobs/5229209492
CI_JOB_TOKEN=[MASKED]
CI_JOB_STARTED_AT=2023-10-05T09:51:03Z
CI_CONCURRENT_ID=9
CI_PROJECT_DESCRIPTION=
CI_PROJECT_CLASSIFICATION_LABEL=
FF_USE_LEGACY_KUBERNETES_EXECUTION_STRATEGY=false
CI_RUNNER_REVISION=2b6048b4
FF_KUBERNETES_HONOR_ENTRYPOINT=false
FF_USE_NEW_SHELL_ESCAPE=false
CI_DEPENDENCY_PROXY_USER=gitlab-ci-token
FF_USE_DYNAMIC_TRACE_FORCE_SEND_INTERVAL=false
FF_DISABLE_UMASK_FOR_DOCKER_EXECUTOR=false
CI_PROJECT_PATH_SLUG=diggerdev-digger-demo
CI_NODE_TOTAL=1
CI_BUILDS_DIR=/builds
CI_JOB_ID=5229209492
CI_PROJECT_REPOSITORY_LANGUAGES=hcl
PATH=/go/bin:/usr/local/go/bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin
FF_SECRET_RESOLVING_FAILS_IF_MISSING=true
CI_PROJECT_ID=44723537
CI=true
GITLAB_CI=true
GITLAB_TOKEN=[MASKED]glpat-
CI_JOB_IMAGE=golang:1.20
CI_COMMIT_BEFORE_SHA=b8c339f048d4d1296ca5791584158744b166df83
CI_PROJECT_TITLE=Digger Demo
CI_SERVER_VERSION_MAJOR=16
CI_CONFIG_PATH=.gitlab-ci.yml
FF_USE_FASTZIP=false
CI_DEPENDENCY_PROXY_SERVER=gitlab.com:443
DOCKER_DRIVER=overlay2
CI_PROJECT_URL=https://gitlab.com/diggerdev/digger-demo
OLDPWD=/go
GOPATH=/go
`

var mergedMergeRequestEnvVars = `
CI_PROJECT_NAMESPACE=diggerdev
GITLAB_USER_ID=13159253
CI_RUNNER_VERSION=16.3.0~beta.108.g2b6048b4
FF_SKIP_NOOP_BUILD_STAGES=true
CI_SERVER_NAME=GitLab
CI_RUNNER_DESCRIPTION=1-green.saas-linux-small-amd64.runners-manager.gitlab.com/default
GITLAB_USER_EMAIL=alexey@digger.dev
CI_SERVER_REVISION=176e0aff712
FF_USE_WINDOWS_LEGACY_PROCESS_STRATEGY=true
CI_RUNNER_EXECUTABLE_ARCH=linux/amd64
CI_PIPELINE_NAME=
CI_REGISTRY_USER=gitlab-ci-token
CI_API_V4_URL=https://gitlab.com/api/v4
CI_REGISTRY_PASSWORD=[MASKED]
CI_RUNNER_SHORT_TOKEN=JLgUopmM
CI_JOB_NAME=print_env
HOSTNAME=runner-jlguopmm-project-44723537-concurrent-0
GITLAB_USER_LOGIN=alexey_digger
CI_PROJECT_NAME=digger-demo
CI_PIPELINE_SOURCE=push
FF_RETRIEVE_POD_WARNING_EVENTS=false
CI_JOB_STATUS=running
CI_PIPELINE_ID=1026558535
FF_DISABLE_POWERSHELL_STDIN=false
CI_COMMIT_REF_SLUG=main
CI_SERVER=yes
FF_SET_PERMISSIONS_BEFORE_CLEANUP=true
CI_COMMIT_SHORT_SHA=ffaf8ce1
CI_JOB_NAME_SLUG=print-env
RUNNER_TEMP_PROJECT_DIR=/builds/diggerdev/digger-demo.tmp
FF_CMD_DISABLE_DELAYED_ERROR_LEVEL_EXPANSION=false
CI_DEPENDENCY_PROXY_GROUP_IMAGE_PREFIX=gitlab.com:443/diggerdev/dependency_proxy/containers
AWS_REGION=us-east-1
PWD=/builds/diggerdev/digger-demo
CI_RUNNER_TAGS=["gce", "east-c", "linux", "ruby", "mysql", "postgres", "mongo", "git-annex", "shared", "docker", "saas-linux-small-amd64"]
CI_PROJECT_PATH=diggerdev/digger-demo
FF_USE_NEW_BASH_EVAL_STRATEGY=false
CI_SERVER_TLS_CA_FILE=/builds/diggerdev/digger-demo.tmp/CI_SERVER_TLS_CA_FILE
CI_DEPENDENCY_PROXY_DIRECT_GROUP_IMAGE_PREFIX=gitlab.com:443/diggerdev/dependency_proxy/containers
CI_COMMIT_REF_PROTECTED=true
FF_USE_POWERSHELL_PATH_RESOLVER=false
CI_API_GRAPHQL_URL=https://gitlab.com/api/graphql
CI_SERVER_VERSION_MINOR=5
CI_COMMIT_SHA=ffaf8ce1eb166a318fbb43e25ce708f37675d174
HOME=/root
FF_NETWORK_PER_BUILD=false
CI_DEPENDENCY_PROXY_PASSWORD=[MASKED]
CI_JOB_TIMEOUT=3600
CI_PROJECT_VISIBILITY=private
CI_CONCURRENT_PROJECT_ID=0
FF_SCRIPT_SECTIONS=false
CI_COMMIT_MESSAGE="Merge branch 'test-staging-changes' into 'main'\n
Update main.tf\n
See merge request diggerdev/digger-demo!44"
DOCKER_TLS_CERTDIR=
CI_SERVER_SHELL_SSH_PORT=22
CI_JOB_JWT_V1=[MASKED]
CI_JOB_JWT_V2=[MASKED]
FF_USE_DIRECT_DOWNLOAD=true
CI_PAGES_DOMAIN=gitlab.io
CI_SERVER_VERSION=16.5.0-pre
FF_USE_POD_ACTIVE_DEADLINE_SECONDS=false
CI_REGISTRY=registry.gitlab.com
CI_SERVER_PORT=443
AWS_SECRET_ACCESS_KEY=[MASKED]
CI_PROJECT_NAMESPACE_ID=12854814
GOLANG_VERSION=1.20.8
FF_USE_IMPROVED_URL_MASKING=false
CI_PAGES_URL=https://diggerdev.gitlab.io/digger-demo
CI_PIPELINE_IID=523
CI_REPOSITORY_URL=https://gitlab-ci-token:[MASKED]@gitlab.com/diggerdev/digger-demo.git
CI_SERVER_URL=https://gitlab.com
FF_ENABLE_BASH_EXIT_CODE_CHECK=false
GITLAB_FEATURES=audit_events,blocked_issues,board_iteration_lists,code_owners,code_review_analytics,contribution_analytics,elastic_search,full_codequality_report,group_activity_analytics,group_bulk_edit,group_webhooks,issuable_default_templates,issue_weights,iterations,ldap_group_sync,member_lock,merge_request_approvers,milestone_charts,multiple_issue_assignees,multiple_ldap_servers,multiple_merge_request_assignees,multiple_merge_request_reviewers,project_merge_request_analytics,protected_refs_for_users,push_rules,repository_mirrors,resource_access_token,seat_link,usage_quotas,visual_review_app,wip_limits,zoekt_code_search,blocked_work_items,description_diffs,send_emails_from_admin_area,repository_size_limit,maintenance_mode,scoped_issue_board,adjourned_deletion_for_projects_and_groups,admin_audit_log,auditor_user,blocking_merge_requests,board_assignee_lists,board_milestone_lists,ci_cd_projects,ci_namespace_catalog,ci_secrets_management,cluster_agents_ci_impersonation,cluster_agents_user_impersonation,cluster_deployments,code_owner_approval_required,code_suggestions,commit_committer_check,commit_committer_name_check,compliance_framework,custom_compliance_frameworks,cross_project_pipelines,custom_file_templates,custom_file_templates_for_namespace,custom_project_templates,cycle_analytics_for_groups,cycle_analytics_for_projects,db_load_balancing,default_branch_protection_restriction_in_groups,default_project_deletion_protection,delete_unconfirmed_users,dependency_proxy_for_packages,disable_name_update_for_users,disable_personal_access_tokens,domain_verification,email_additional_text,epics,extended_audit_events,external_authorization_service_api_management,feature_flags_related_issues,feature_flags_code_references,file_locks,geo,generic_alert_fingerprinting,git_two_factor_enforcement,github_integration,group_allowed_email_domains,group_coverage_reports,group_forking_protection,group_milestone_project_releases,group_project_templates,group_repository_analytics,group_saml,group_scoped_ci_variables,group_wikis,ide_schema_config,incident_metric_upload,incident_sla,instance_level_scim,issues_analytics,jira_issues_integration,ldap_group_sync_filter,merge_pipelines,merge_request_performance_metrics,admin_merge_request_approvers_rules,merge_trains,metrics_reports,multiple_alert_http_integrations,multiple_approval_rules,multiple_group_issue_boards,object_storage,microsoft_group_sync,operations_dashboard,package_forwarding,pages_size_limit,pages_multiple_versions,productivity_analytics,project_aliases,protected_environments,reject_non_dco_commits,reject_unsigned_commits,remote_development,saml_group_sync,service_accounts,scoped_labels,smartcard_auth,ssh_certificates,swimlanes,target_branch_rules,type_of_work_analytics,minimal_access_role,unprotection_restrictions,ci_project_subscriptions,incident_timeline_view,oncall_schedules,escalation_policies,export_user_permissions,zentao_issues_integration,coverage_check_approval_rule,issuable_resource_links,group_protected_branches,group_level_merge_checks_setting,oidc_client_groups_claim,disable_deleting_account_for_users,group_ip_restriction,password_complexity,enterprise_templates,git_abuse_rate_limit,required_ci_templates,runner_maintenance_note,runner_performance_insights,runner_upgrade_management
CI_COMMIT_DESCRIPTION="\n
Update main.tf\n
See merge request diggerdev/digger-demo!44"
FF_USE_ADVANCED_POD_SPEC_CONFIGURATION=false
CI_TEMPLATE_REGISTRY_HOST=registry.gitlab.com
CI_JOB_STAGE=digger
CI_PIPELINE_URL=https://gitlab.com/diggerdev/digger-demo/-/pipelines/1026558535
CI_DEFAULT_BRANCH=main
CI_SERVER_VERSION_PATCH=0
CI_COMMIT_TITLE=Merge branch 'test-staging-changes' into 'main'
CI_PROJECT_ROOT_NAMESPACE=diggerdev
FF_ENABLE_JOB_CLEANUP=false
FF_RESOLVE_FULL_TLS_CHAIN=true
GITLAB_USER_NAME=Alexey Skriptsov
CI_PROJECT_DIR=/builds/diggerdev/digger-demo
SHLVL=1
CI_RUNNER_ID=12270845
CI_PIPELINE_CREATED_AT=2023-10-05T09:45:55Z
CI_COMMIT_TIMESTAMP=2023-10-05T09:45:54+00:00
AWS_ACCESS_KEY_ID=AKIATBSMGQ3XRUHRM725
CI_DISPOSABLE_ENVIRONMENT=true
CI_SERVER_SHELL_SSH_HOST=gitlab.com
CI_JOB_JWT=[MASKED]
CI_REGISTRY_IMAGE=registry.gitlab.com/diggerdev/digger-demo
CI_SERVER_PROTOCOL=https
CI_COMMIT_AUTHOR=Alexey Skriptsov <alexey@digger.dev>
FF_POSIXLY_CORRECT_ESCAPES=false
CI_COMMIT_REF_NAME=main
CI_SERVER_HOST=gitlab.com
CI_JOB_URL=https://gitlab.com/diggerdev/digger-demo/-/jobs/5229168106
CI_JOB_TOKEN=[MASKED]
CI_JOB_STARTED_AT=2023-10-05T09:45:57Z
CI_CONCURRENT_ID=143
CI_PROJECT_DESCRIPTION=
CI_COMMIT_BRANCH=main
CI_PROJECT_CLASSIFICATION_LABEL=
FF_USE_LEGACY_KUBERNETES_EXECUTION_STRATEGY=false
CI_RUNNER_REVISION=2b6048b4
FF_KUBERNETES_HONOR_ENTRYPOINT=false
FF_USE_NEW_SHELL_ESCAPE=false
CI_DEPENDENCY_PROXY_USER=gitlab-ci-token
FF_USE_DYNAMIC_TRACE_FORCE_SEND_INTERVAL=false
FF_DISABLE_UMASK_FOR_DOCKER_EXECUTOR=false
CI_PROJECT_PATH_SLUG=diggerdev-digger-demo
CI_NODE_TOTAL=1
CI_BUILDS_DIR=/builds
CI_JOB_ID=5229168106
CI_PROJECT_REPOSITORY_LANGUAGES=hcl
PATH=/go/bin:/usr/local/go/bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin
FF_SECRET_RESOLVING_FAILS_IF_MISSING=true
CI_PROJECT_ID=44723537
CI=true
GITLAB_CI=true
GITLAB_TOKEN=[MASKED]glpat-
CI_JOB_IMAGE=golang:1.20
CI_COMMIT_BEFORE_SHA=e84723c6324c9d35b779b7a61f15cd9af28c8247
CI_PROJECT_TITLE=Digger Demo
CI_SERVER_VERSION_MAJOR=16
CI_CONFIG_PATH=.gitlab-ci.yml
FF_USE_FASTZIP=false
CI_DEPENDENCY_PROXY_SERVER=gitlab.com:443
DOCKER_DRIVER=overlay2
CI_PROJECT_URL=https://gitlab.com/diggerdev/digger-demo
OLDPWD=/go
GOPATH=/go
_=/usr/bin/env
`

func TestParseGitLabContext(t *testing.T) {
	t.Setenv("CI_PIPELINE_SOURCE", "push")
	t.Setenv("CI_PIPELINE_ID", "1")
	t.Setenv("CI_PIPELINE_IID", "2")

	context, err := ParseGitLabContext()
	assert.NoError(t, err)
	assert.NotNil(t, context)
	assert.Equal(t, PipelineSourceTypePush, context.PipelineSource)
	assert.Equal(t, 1, *context.PipelineId)
	assert.Equal(t, 2, *context.PipelineIId)
	assert.Nil(t, context.MergeRequestId)
	assert.Nil(t, context.MergeRequestIId)
}

func TestOpenMergeRequestEvent(t *testing.T) {
	t.Setenv("CI_PIPELINE_SOURCE", "push")
	t.Setenv("CI_PIPELINE_ID", "1")
	t.Setenv("CI_PIPELINE_IID", "2")

	context, err := ParseGitLabContext()
	assert.NoError(t, err)
	assert.NotNil(t, context)
	assert.Equal(t, PipelineSourceTypePush, context.PipelineSource)
	assert.Equal(t, 1, *context.PipelineId)
	assert.Equal(t, 2, *context.PipelineIId)
	assert.Nil(t, context.MergeRequestId)
	assert.Nil(t, context.MergeRequestIId)
}

// TestNewMergeRequestGitLabContextEvent  in gitlab there is no difference between new Merge Request and updated Merge Request
func TestNewMergeRequestGitLabContextEvent(t *testing.T) {
	result, err := godotenv.Parse(strings.NewReader(newMergeRequestEnvVars))
	assert.NoError(t, err)
	for k, v := range result {
		err := os.Setenv(k, v)
		assert.NoError(t, err)
	}
	gitLabContext, err := ParseGitLabContext()
	assert.NoError(t, err)
	assert.NotNil(t, gitLabContext)
	assert.NotNil(t, gitLabContext.MergeRequestId)
	assert.NotNil(t, gitLabContext.MergeRequestIId)
	assert.Equal(t, 1, len(gitLabContext.OpenMergeRequests))
	assert.Equal(t, PipelineSourceTypeMergeRequestEvent, gitLabContext.PipelineSource)
	assert.Equal(t, MergeRequestCreatedOrUpdated, gitLabContext.EventType)
	assert.Contains(t, *gitLabContext.CommitMessage, "See merge request")
	assert.Equal(t, "test-dev", *gitLabContext.MergeRequestSourceBranchName)
	assert.Equal(t, "main", *gitLabContext.MergeRequestTargetBranchName)

	diggerCfg := `
projects:
- name: prod
  dir: /prod
`
	diggerConfig, _, _, err := configuration.LoadDiggerConfigFromString(diggerCfg, "./")
	assert.NoError(t, err)
	gitlabService := GitLabMockService{}
	gitlabService.ChangedFiles = []string{"prod/main.tf"}
	impactedProjects, err := ProcessGitLabEvent(gitLabContext, diggerConfig, &gitlabService)
	assert.NotNil(t, impactedProjects)

	jobs, _, err := ConvertGitLabEventToCommands(gitLabContext, impactedProjects, diggerConfig.Workflows)
	assert.NoError(t, err)
	assert.NotNil(t, jobs)
}

func TestMergedMergeRequestGitLabContextEvent(t *testing.T) {
	result, err := godotenv.Parse(strings.NewReader(mergedMergeRequestEnvVars))
	assert.NoError(t, err)
	for k, v := range result {
		err := os.Setenv(k, v)
		assert.NoError(t, err)
	}
	gitLabContext, err := ParseGitLabContext()
	assert.NoError(t, err)
	assert.Nil(t, gitLabContext.OpenMergeRequests)
	assert.NotNil(t, gitLabContext)
	assert.Nil(t, gitLabContext.MergeRequestId)
	assert.Nil(t, gitLabContext.MergeRequestIId)
	assert.Equal(t, PipelineSourceTypePush, gitLabContext.PipelineSource)
	assert.Equal(t, MergeRequestMerged, gitLabContext.EventType)
	assert.Contains(t, *gitLabContext.CommitMessage, "See merge request")
	assert.Nil(t, gitLabContext.MergeRequestSourceBranchName)
	assert.Nil(t, gitLabContext.MergeRequestTargetBranchName)
	//gitlabToken := "test"
	//gitlabService, err := NewGitLabService(gitlabToken, gitLabContext)
	//assert.NoError(t, err)
	//assert.Nil(t, gitlabService)

	diggerCfg := `
projects:
- name: prod
  dir: /prod
`
	diggerConfig, _, _, err := configuration.LoadDiggerConfigFromString(diggerCfg, "./")
	assert.NoError(t, err)
	gitlabService := GitLabMockService{}
	gitlabService.ChangedFiles = []string{"prod/main.tf"}
	impactedProjects, err := ProcessGitLabEvent(gitLabContext, diggerConfig, &gitlabService)
	assert.NoError(t, err)
	assert.NotNil(t, impactedProjects)
	//assert.NotNil(t, requestedProject)

	jobs, _, err := ConvertGitLabEventToCommands(gitLabContext, impactedProjects, diggerConfig.Workflows)
	assert.NoError(t, err)
	assert.NotNil(t, jobs)

}
