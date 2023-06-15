package policy

import (
	"reflect"
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

func TestDiggerPolicyChecker_Check(t *testing.T) {
	type fields struct {
		PolicyProvider PolicyProvider
	}
	type args struct {
		input interface{}
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    bool
		wantErr bool
	}{
		{
			name: "test opa example",
			fields: fields{
				PolicyProvider: &OpaExamplePolicyProvider{},
			},
			args: args{
				input: map[string]interface{}{
					"user":   "alice",
					"action": "read",
					"object": "server123",
				},
			},
			want:    true,
			wantErr: false,
		},
		{
			name: "test digger example",
			fields: fields{
				PolicyProvider: &DiggerExamplePolicyProvider{},
			},
			args: args{
				input: map[string]interface{}{
					"user":   "motatoes",
					"action": "digger plan",
				},
			},
			want:    true,
			wantErr: false,
		},
		{
			name: "test digger example 2",
			fields: fields{
				PolicyProvider: &DiggerExamplePolicyProvider{},
			},
			args: args{
				input: map[string]interface{}{
					"user":   "Spartakovic",
					"action": "digger unlock",
				},
			},
			want:    false,
			wantErr: false,
		},
		{
			name: "test digger example 3",
			fields: fields{
				PolicyProvider: &DiggerExamplePolicyProvider{},
			},
			args: args{
				input: map[string]interface{}{
					"user":   "rando",
					"action": "digger apply",
				},
			},
			want:    false,
			wantErr: false,
		},
		{
			name: "test digger example 4",
			fields: fields{
				PolicyProvider: &DiggerExamplePolicyProvider2{},
			},
			args: args{
				input: map[string]interface{}{
					"user":   "motatoes",
					"action": "digger plan",
				},
			},
			want:    true,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := &DiggerPolicyChecker{
				PolicyProvider: tt.fields.PolicyProvider,
			}
			got, err := p.Check(tt.name, tt.name, tt.args.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("DiggerPolicyChecker.Check() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("DiggerPolicyChecker.Check() got = %v, want %v", got, tt.want)
			}
			if !reflect.DeepEqual(got1, tt.want1) {
				t.Errorf("DiggerPolicyChecker.Check() got1 = %v, want %v", got1, tt.want1)
			}
		})
	}
}
