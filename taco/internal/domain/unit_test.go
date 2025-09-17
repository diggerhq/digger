package domain

import "testing"

func TestValidateUnitID(t *testing.T) {
    tests := []struct{
        name string
        id string
        wantErr bool
    }{
        {name: "valid simple ID", id: "my-unit", wantErr: false},
        {name: "valid nested ID", id: "my-project/prod/vpc", wantErr: false},
        {name: "empty ID", id: "", wantErr: true},
        {name: "ID with ..", id: "my-project/../evil", wantErr: true},
        {name: "just slashes", id: "///", wantErr: true},
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            err := ValidateUnitID(tt.id)
            if (err != nil) != tt.wantErr {
                t.Errorf("ValidateUnitID() error = %v, wantErr %v", err, tt.wantErr)
            }
        })
    }
}

func TestNormalizeUnitID(t *testing.T) {
    tests := []struct{
        name string
        id string
        want string
    }{
        {name: "simple ID", id: "my-unit", want: "my-unit"},
        {name: "leading/trailing slashes", id: "/my-unit/", want: "my-unit"},
        {name: "multiple slashes", id: "my//project///prod", want: "my/project/prod"},
        {name: "complex path", id: "///my/project//prod/vpc///", want: "my/project/prod/vpc"},
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got := NormalizeUnitID(tt.id)
            if got != tt.want {
                t.Errorf("NormalizeUnitID() = %v, want %v", got, tt.want)
            }
        })
    }
}

func TestFilterUnitsByPrefix(t *testing.T) {
    units := []*Unit{
        {ID: "project-a/dev/vpc"},
        {ID: "project-a/prod/vpc"},
        {ID: "project-b/dev/vpc"},
        {ID: "project-b/prod/vpc"},
        {ID: "global/dns"},
    }
    tests := []struct{
        name string
        prefix string
        want int
    }{
        {name: "empty prefix", prefix: "", want: 5},
        {name: "project-a prefix", prefix: "project-a", want: 2},
        {name: "project-b/prod prefix", prefix: "project-b/prod", want: 1},
        {name: "non-existent prefix", prefix: "project-c", want: 0},
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            filtered := FilterUnitsByPrefix(units, tt.prefix)
            if len(filtered) != tt.want {
                t.Errorf("FilterUnitsByPrefix() returned %d units, want %d", len(filtered), tt.want)
            }
        })
    }
}

