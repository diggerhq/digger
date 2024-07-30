package spec

type GetSpecPayload struct {
	Command      string `json:"command"`
	RepoFullName string `json:"repo_full_name"`
	Actor        string `json:"actor"`
	//DefaultBranch string `json:"default_branch"`
	//PrBranch      string `json:"pr_branch"`
	DiggerConfig string `json:"digger_config"`
	Project      string `json:"project"`
}

func (p GetSpecPayload) ToMapStruct() map[string]interface{} {
	return map[string]interface{}{
		"command":        p.Command,
		"repo_full_name": p.RepoFullName,
		"actor":          p.Actor,
		//"default_branch": p.DefaultBranch,
		//"pr_branch":      p.PrBranch,
		"digger_config": p.DiggerConfig,
		"project":       p.Project,
	}
}
