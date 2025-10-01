package tfe

import "fmt"

type ResourceType string

func (e ResourceType) String() string {
	return string(e)
}

const (
	OrganizationType ResourceType = "org"
	WorkspaceType    ResourceType = "ws"
	AgentPoolType    ResourceType = "apool"
)

type TfeResourceIdentifier struct {
	resourceType ResourceType
	id           string
}

func (id TfeResourceIdentifier) String() string {
	return fmt.Sprintf("%v-%v", id.resourceType, id.id)
}

func NewTfeResourceIdentifier(resourceType ResourceType, id string) TfeResourceIdentifier {
	return TfeResourceIdentifier{
		resourceType: resourceType,
		id:           id,
	}
}
