package policy

import (
	"digger/pkg/utils"
	"testing"
)

type OpaExamplePolicyProvider struct {
}

func (s *OpaExamplePolicyProvider) GetPolicy(_ string, _ string) (string, error) {
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

func (s *DiggerExamplePolicyProvider) GetPolicy(_ string, _ string) (string, error) {
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

func (s *DiggerExamplePolicyProvider) GetOrganisation() string {
	return "ORGANISATIONDIGGER"
}

type DiggerExamplePolicyProvider2 struct {
}

func (s *DiggerExamplePolicyProvider2) GetPolicy(_ string, _ string) (string, error) {
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

func (s *DiggerExamplePolicyProvider2) GetOrganisation() string {
	return "ORGANISATIONDIGGER"
}

func TestDiggerPolicyChecker_Check(t *testing.T) {
	type fields struct {
		PolicyProvider PolicyProvider
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
			got, err := p.Check(ciService, tt.organisation, tt.name, tt.name, tt.command, tt.requestedBy)
			if (err != nil) != tt.wantErr {
				t.Errorf("DiggerPolicyChecker.Check() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("DiggerPolicyChecker.Check() got = %v, want %v", got, tt.want)
			}
		})
	}
}
