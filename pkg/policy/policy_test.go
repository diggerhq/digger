package policy

import (
	"testing"

	"github.com/diggerhq/digger/pkg/core/policy"
	"github.com/diggerhq/digger/pkg/utils"
)

type OpaExamplePolicyProvider struct {
}

func (s *OpaExamplePolicyProvider) GetAccessPolicy(_ string, _ string) (string, error) {
	return "package digger\n" +
		"\n" +
		"# user-role assignments\n" +
		"user_roles := {\n" +
		"    \"alice\": [\"engineering\", \"webdev\"],\n" +
		"    \"bob\": [\"hr\"]\n" +
		"}\n" +
		"\n" +
		"# role-permissions assignments\n" +
		"role_permissions := {\n" +
		"    \"engineering\": [{\"action\": \"read\",  \"object\": \"server123\"}],\n" +
		"    \"webdev\":      [{\"action\": \"read\",  \"object\": \"server123\"},\n" +
		"                    {\"action\": \"write\", \"object\": \"server123\"}],\n" +
		"    \"hr\":          [{\"action\": \"read\",  \"object\": \"database456\"}]\n" +
		"}\n" +
		"\n" +
		"# logic that implements RBAC.\n" +
		"default allow = false\n" +
		"allow {\n" +
		"    # lookup the list of roles for the user\n" +
		"    roles := user_roles[input.user]\n" +
		"    # for each role in that list\n" +
		"    r := roles[_]\n" +
		"    # lookup the permissions list for role r\n" +
		"    permissions := role_permissions[r]\n" +
		"    # for each permission\n" +
		"    p := permissions[_]\n" +
		"    # check if the permission granted to r matches the user's request\n" +
		"    p == {\"action\": input.action, \"object\": input.object}\n" +
		"}", nil
}

func (s *OpaExamplePolicyProvider) GetOrganisation() string {
	return "ORGANISATIONDIGGER"
}

type DiggerExamplePolicyProvider struct {
}

func (s *DiggerExamplePolicyProvider) GetAccessPolicy(_ string, _ string, _ string) (string, error) {
	return "package digger\n" +
		"\n" +
		"user_permissions := {\n" +
		"    \"motatoes\": [\"digger plan\"], \"Spartakovic\": [\"digger plan\", \"digger apply\"]\n" +
		"}\n" +
		"\n" +
		"default allow = false\n" +
		"allow {\n" +
		"    permissions := user_permissions[input.user]\n" +
		"    p := permissions[_]\n" +
		"    p == input.action\n" +
		"}\n" +
		"", nil
}

func (s *DiggerExamplePolicyProvider) GetPlanPolicy(_ string, _ string, _ string) (string, error) {
	return "package digger\n", nil
}

func (s *DiggerExamplePolicyProvider) GetDriftPolicy() (string, error) {
	return "package digger\n", nil
}

func (s *DiggerExamplePolicyProvider) GetOrganisation() string {
	return "ORGANISATIONDIGGER"
}

type DiggerExamplePolicyProvider2 struct {
}

func (s *DiggerExamplePolicyProvider2) GetAccessPolicy(_ string, _ string, _ string) (string, error) {
	return "package digger\n" +
		"\n" +
		"user_permissions := {\n" +
		"    \"motatoes\": [\"digger plan\"], \"Spartakovic\": [\"digger plan\", \"digger apply\"]\n" +
		"}\n" +
		"\n" +
		"default allow = false\n" +
		"allow {\n" +
		"    permissions := user_permissions[input.user]\n" +
		"    p := permissions[_]\n" +
		"    p == input.action\n" +
		"}\n" +
		"allow {\n" +
		"    1 == 1\n" +
		"}\n" +
		"", nil
}

func (s *DiggerExamplePolicyProvider2) GetPlanPolicy(_ string, _ string, _ string) (string, error) {
	return "package digger\n\ndeny[sprintf(message, [resource.address])] {\n  message := \"Cannot create EC2 instances!\"\n  resource := input.terraform.resource_changes[_]\n  resource.change.actions[_] == \"create\"\n  resource[type] == \"aws_instance\"\n}\n", nil
}

func (s *DiggerExamplePolicyProvider2) GetDriftPolicy() (string, error) {
	return "package digger\n", nil
}

func (s *DiggerExamplePolicyProvider2) GetOrganisation() string {
	return "ORGANISATIONDIGGER"
}

func TestDiggerAccessPolicyChecker_Check(t *testing.T) {
	type fields struct {
		PolicyProvider policy.Provider
	}
	type args struct {
		input interface{}
	}
	tests := []struct {
		name         string
		organisation string
		fields       fields
		args         args
		want         bool
		wantErr      bool
		command      string
		requestedBy  string
	}{
		{
			name: "test digger example",
			fields: fields{
				PolicyProvider: &DiggerExamplePolicyProvider{},
			},
			want:        true,
			wantErr:     false,
			command:     "digger plan",
			requestedBy: "motatoes",
		},
		{
			name: "test digger example 2",
			fields: fields{
				PolicyProvider: &DiggerExamplePolicyProvider{},
			},
			want:        false,
			wantErr:     false,
			command:     "digger unlock",
			requestedBy: "Spartakovic",
		},
		{
			name: "test digger example 3",
			fields: fields{
				PolicyProvider: &DiggerExamplePolicyProvider{},
			},
			want:        false,
			wantErr:     false,
			command:     "digger apply",
			requestedBy: "rando",
		},
		{
			name: "test digger example 4",
			fields: fields{
				PolicyProvider: &DiggerExamplePolicyProvider2{},
			},
			want:        true,
			wantErr:     false,
			command:     "digger plan",
			requestedBy: "motatoes",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var p = &DiggerPolicyChecker{
				PolicyProvider: tt.fields.PolicyProvider,
			}
			ciService := utils.MockPullRequestManager{Teams: []string{"engineering"}}
			got, err := p.CheckAccessPolicy(ciService, nil, tt.organisation, tt.name, tt.name, tt.command, nil, tt.requestedBy, []string{})
			if (err != nil) != tt.wantErr {
				t.Errorf("DiggerPolicyChecker.CheckAccessPolicy() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("DiggerPolicyChecker.CheckAccessPolicy() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDiggerPlanPolicyChecker_Check(t *testing.T) {
	type fields struct {
		PolicyProvider policy.Provider
	}
	type args struct {
		input interface{}
	}
	tests := []struct {
		name           string
		planJsonOutput string
		fields         fields
		args           args
		want           bool
		wantErr        bool
	}{
		{
			name:           "test digger disallow aws instance",
			planJsonOutput: "{\"format_version\":\"1.2\",\"terraform_version\":\"1.5.2\",\"planned_values\":{\"root_module\":{\"resources\":[{\"address\":\"aws_instance.web\",\"mode\":\"managed\",\"type\":\"aws_instance\",\"name\":\"web\",\"provider_name\":\"registry.terraform.io/hashicorp/aws\",\"schema_version\":1,\"values\":{\"ami\":\"zzz\",\"credit_specification\":[],\"get_password_data\":false,\"hibernation\":null,\"instance_type\":\"t2.micro\",\"launch_template\":[],\"source_dest_check\":true,\"tags\":null,\"timeouts\":null,\"user_data_replace_on_change\":false,\"volume_tags\":null},\"sensitive_values\":{\"capacity_reservation_specification\":[],\"cpu_options\":[],\"credit_specification\":[],\"ebs_block_device\":[],\"enclave_options\":[],\"ephemeral_block_device\":[],\"instance_market_options\":[],\"ipv6_addresses\":[],\"launch_template\":[],\"maintenance_options\":[],\"metadata_options\":[],\"network_interface\":[],\"private_dns_name_options\":[],\"root_block_device\":[],\"secondary_private_ips\":[],\"security_groups\":[],\"tags_all\":{},\"vpc_security_group_ids\":[]}},{\"address\":\"null_resource.test4\",\"mode\":\"managed\",\"type\":\"null_resource\",\"name\":\"test4\",\"provider_name\":\"registry.terraform.io/hashicorp/null\",\"schema_version\":0,\"values\":{\"triggers\":null},\"sensitive_values\":{}}]}},\"resource_changes\":[{\"address\":\"aws_instance.web\",\"mode\":\"managed\",\"type\":\"aws_instance\",\"name\":\"web\",\"provider_name\":\"registry.terraform.io/hashicorp/aws\",\"change\":{\"actions\":[\"create\"],\"before\":null,\"after\":{\"ami\":\"zzz\",\"credit_specification\":[],\"get_password_data\":false,\"hibernation\":null,\"instance_type\":\"t2.micro\",\"launch_template\":[],\"source_dest_check\":true,\"tags\":null,\"timeouts\":null,\"user_data_replace_on_change\":false,\"volume_tags\":null},\"after_unknown\":{\"arn\":true,\"associate_public_ip_address\":true,\"availability_zone\":true,\"capacity_reservation_specification\":true,\"cpu_core_count\":true,\"cpu_options\":true,\"cpu_threads_per_core\":true,\"credit_specification\":[],\"disable_api_stop\":true,\"disable_api_termination\":true,\"ebs_block_device\":true,\"ebs_optimized\":true,\"enclave_options\":true,\"ephemeral_block_device\":true,\"host_id\":true,\"host_resource_group_arn\":true,\"iam_instance_profile\":true,\"id\":true,\"instance_initiated_shutdown_behavior\":true,\"instance_lifecycle\":true,\"instance_market_options\":true,\"instance_state\":true,\"ipv6_address_count\":true,\"ipv6_addresses\":true,\"key_name\":true,\"launch_template\":[],\"maintenance_options\":true,\"metadata_options\":true,\"monitoring\":true,\"network_interface\":true,\"outpost_arn\":true,\"password_data\":true,\"placement_group\":true,\"placement_partition_number\":true,\"primary_network_interface_id\":true,\"private_dns\":true,\"private_dns_name_options\":true,\"private_ip\":true,\"public_dns\":true,\"public_ip\":true,\"root_block_device\":true,\"secondary_private_ips\":true,\"security_groups\":true,\"spot_instance_request_id\":true,\"subnet_id\":true,\"tags_all\":true,\"tenancy\":true,\"user_data\":true,\"user_data_base64\":true,\"vpc_security_group_ids\":true},\"before_sensitive\":false,\"after_sensitive\":{\"capacity_reservation_specification\":[],\"cpu_options\":[],\"credit_specification\":[],\"ebs_block_device\":[],\"enclave_options\":[],\"ephemeral_block_device\":[],\"instance_market_options\":[],\"ipv6_addresses\":[],\"launch_template\":[],\"maintenance_options\":[],\"metadata_options\":[],\"network_interface\":[],\"private_dns_name_options\":[],\"root_block_device\":[],\"secondary_private_ips\":[],\"security_groups\":[],\"tags_all\":{},\"vpc_security_group_ids\":[]}}},{\"address\":\"null_resource.test4\",\"mode\":\"managed\",\"type\":\"null_resource\",\"name\":\"test4\",\"provider_name\":\"registry.terraform.io/hashicorp/null\",\"change\":{\"actions\":[\"create\"],\"before\":null,\"after\":{\"triggers\":null},\"after_unknown\":{\"id\":true},\"before_sensitive\":false,\"after_sensitive\":{}}}],\"digger_config\":{\"provider_config\":{\"aws\":{\"name\":\"aws\",\"full_name\":\"registry.terraform.io/hashicorp/aws\"},\"null\":{\"name\":\"null\",\"full_name\":\"registry.terraform.io/hashicorp/null\"}},\"root_module\":{\"resources\":[{\"address\":\"aws_instance.web\",\"mode\":\"managed\",\"type\":\"aws_instance\",\"name\":\"web\",\"provider_config_key\":\"aws\",\"expressions\":{\"ami\":{\"constant_value\":\"zzz\"},\"instance_type\":{\"constant_value\":\"t2.micro\"}},\"schema_version\":1},{\"address\":\"null_resource.test4\",\"mode\":\"managed\",\"type\":\"null_resource\",\"name\":\"test4\",\"provider_config_key\":\"null\",\"schema_version\":0}]}},\"timestamp\":\"2023-07-19T14:55:44Z\"}",
			fields: fields{
				PolicyProvider: &DiggerExamplePolicyProvider2{},
			},
			want:    false,
			wantErr: false,
		},
		{
			name:           "test digger allow when there is no aws instance",
			planJsonOutput: "{\"format_version\":\"1.2\",\"terraform_version\":\"1.5.2\",\"planned_values\":{\"root_module\":{\"resources\":[{\"address\":\"not_an_instance.web\",\"mode\":\"managed\",\"type\":\"not_an_instance\",\"name\":\"web\",\"provider_name\":\"registry.terraform.io/hashicorp/aws\",\"schema_version\":1,\"values\":{\"ami\":\"zzz\",\"credit_specification\":[],\"get_password_data\":false,\"hibernation\":null,\"instance_type\":\"t2.micro\",\"launch_template\":[],\"source_dest_check\":true,\"tags\":null,\"timeouts\":null,\"user_data_replace_on_change\":false,\"volume_tags\":null},\"sensitive_values\":{\"capacity_reservation_specification\":[],\"cpu_options\":[],\"credit_specification\":[],\"ebs_block_device\":[],\"enclave_options\":[],\"ephemeral_block_device\":[],\"instance_market_options\":[],\"ipv6_addresses\":[],\"launch_template\":[],\"maintenance_options\":[],\"metadata_options\":[],\"network_interface\":[],\"private_dns_name_options\":[],\"root_block_device\":[],\"secondary_private_ips\":[],\"security_groups\":[],\"tags_all\":{},\"vpc_security_group_ids\":[]}},{\"address\":\"null_resource.test4\",\"mode\":\"managed\",\"type\":\"null_resource\",\"name\":\"test4\",\"provider_name\":\"registry.terraform.io/hashicorp/null\",\"schema_version\":0,\"values\":{\"triggers\":null},\"sensitive_values\":{}}]}},\"resource_changes\":[{\"address\":\"not_an_instance.lala\",\"mode\":\"managed\",\"type\":\"not_an_instance\",\"name\":\"web\",\"provider_name\":\"registry.terraform.io/hashicorp/aws\",\"change\":{\"actions\":[\"create\"],\"before\":null,\"after\":{\"ami\":\"zzz\",\"credit_specification\":[],\"get_password_data\":false,\"hibernation\":null,\"instance_type\":\"t2.micro\",\"launch_template\":[],\"source_dest_check\":true,\"tags\":null,\"timeouts\":null,\"user_data_replace_on_change\":false,\"volume_tags\":null},\"after_unknown\":{\"arn\":true,\"associate_public_ip_address\":true,\"availability_zone\":true,\"capacity_reservation_specification\":true,\"cpu_core_count\":true,\"cpu_options\":true,\"cpu_threads_per_core\":true,\"credit_specification\":[],\"disable_api_stop\":true,\"disable_api_termination\":true,\"ebs_block_device\":true,\"ebs_optimized\":true,\"enclave_options\":true,\"ephemeral_block_device\":true,\"host_id\":true,\"host_resource_group_arn\":true,\"iam_instance_profile\":true,\"id\":true,\"instance_initiated_shutdown_behavior\":true,\"instance_lifecycle\":true,\"instance_market_options\":true,\"instance_state\":true,\"ipv6_address_count\":true,\"ipv6_addresses\":true,\"key_name\":true,\"launch_template\":[],\"maintenance_options\":true,\"metadata_options\":true,\"monitoring\":true,\"network_interface\":true,\"outpost_arn\":true,\"password_data\":true,\"placement_group\":true,\"placement_partition_number\":true,\"primary_network_interface_id\":true,\"private_dns\":true,\"private_dns_name_options\":true,\"private_ip\":true,\"public_dns\":true,\"public_ip\":true,\"root_block_device\":true,\"secondary_private_ips\":true,\"security_groups\":true,\"spot_instance_request_id\":true,\"subnet_id\":true,\"tags_all\":true,\"tenancy\":true,\"user_data\":true,\"user_data_base64\":true,\"vpc_security_group_ids\":true},\"before_sensitive\":false,\"after_sensitive\":{\"capacity_reservation_specification\":[],\"cpu_options\":[],\"credit_specification\":[],\"ebs_block_device\":[],\"enclave_options\":[],\"ephemeral_block_device\":[],\"instance_market_options\":[],\"ipv6_addresses\":[],\"launch_template\":[],\"maintenance_options\":[],\"metadata_options\":[],\"network_interface\":[],\"private_dns_name_options\":[],\"root_block_device\":[],\"secondary_private_ips\":[],\"security_groups\":[],\"tags_all\":{},\"vpc_security_group_ids\":[]}}},{\"address\":\"null_resource.test4\",\"mode\":\"managed\",\"type\":\"null_resource\",\"name\":\"test4\",\"provider_name\":\"registry.terraform.io/hashicorp/null\",\"change\":{\"actions\":[\"create\"],\"before\":null,\"after\":{\"triggers\":null},\"after_unknown\":{\"id\":true},\"before_sensitive\":false,\"after_sensitive\":{}}}],\"digger_config\":{\"provider_config\":{\"aws\":{\"name\":\"aws\",\"full_name\":\"registry.terraform.io/hashicorp/aws\"},\"null\":{\"name\":\"null\",\"full_name\":\"registry.terraform.io/hashicorp/null\"}},\"root_module\":{\"resources\":[{\"address\":\"aws_instance.web\",\"mode\":\"managed\",\"type\":\"aws_instance\",\"name\":\"web\",\"provider_config_key\":\"aws\",\"expressions\":{\"ami\":{\"constant_value\":\"zzz\"},\"instance_type\":{\"constant_value\":\"t2.micro\"}},\"schema_version\":1},{\"address\":\"null_resource.test4\",\"mode\":\"managed\",\"type\":\"null_resource\",\"name\":\"test4\",\"provider_config_key\":\"null\",\"schema_version\":0}]}},\"timestamp\":\"2023-07-19T14:55:44Z\"}",
			fields: fields{
				PolicyProvider: &DiggerExamplePolicyProvider2{},
			},
			want:    true,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var p = &DiggerPolicyChecker{
				PolicyProvider: tt.fields.PolicyProvider,
			}
			got, _, err := p.CheckPlanPolicy("", "", "", tt.planJsonOutput)
			if (err != nil) != tt.wantErr {
				t.Errorf("DiggerPolicyChecker.CheckPlanPolicy() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("DiggerPolicyChecker.CheckPlanPolicy() got = %v, want %v", got, tt.want)
			}
		})
	}
}
