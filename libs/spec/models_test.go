package spec

const stubTest = `
{
	"job_id": "asdf2321",
	"comment_id": "123",
	"run_name": "asdf",
	"job": {
		"job_type": "plan",
		"projectName": "abc",
		"projectDir": "asd",
		"projectWorkspace": "",
		"terragrunt": false,
		"opentofu": false,
		"commands": [
			"digger plan"
		],
		"applyStage": {
			"steps": [
				{
					"action": "init",
					"extra_args": []
				},
				{
					"action": "apply",
					"extra_args": []
				}
			]
		},
		"planStage": {
			"steps": [
				{
					"action": "init",
					"extra_args": []
				},
				{
					"action": "plan",
					"extra_args": []
				}
			]
		},
		"commit": "shasha",
		"branch": "",
		"eventName": "pull_request",
		"requestedBy": "motatoes",
		"namespace": "friendly",
		"runEnvVars": {},
		"stateEnvVars": {},
		"commandEnvVars": {},
		"aws_role_region": "us-east-1",
		"state_role_name": "",
		"command_role_name": "",
		"backend_hostname": "",
		"backend_organisation_hostname": "",
		"backend_token": ""
	},
	"reporter": {},
	"lock": {},
	"backendapi": {},
	"vcs": {},
	"policy_provider": {}
}
`
