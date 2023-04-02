package domain

type ProjectCommand struct {
	Name       string
	WorkingDir string
	Actions    []Action
	Runner     string
}
