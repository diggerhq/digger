package tfe

type LoginSpec struct {
	Client     string   `json:"client"`
	GrantTypes []string `json:"grant_types"`
	Authz      string   `json:"authz"`
	Token      string   `json:"token"`
	Ports      []int    `json:"ports"`
}

type WellKnownSpec struct {
	Login           LoginSpec `json:"login.v1"`
	Modules         string    `json:"modules.v1"`
	MessageOfTheDay string    `json:"motd.v1"`
	State           string    `json:"state.v2"`
	TfeApiV2        string    `json:"tfe.v2"`
	TfeApiV21       string    `json:"tfe.v2.1"`
	TfeApiV22       string    `json:"tfe.v2.2"`
}
