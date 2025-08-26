package rbac

// Action represents a permissioned operation on a state.
type Action string

const (
    ActionStateRead  Action = "state.read"
    ActionStateWrite Action = "state.write"
    ActionStateLock  Action = "state.lock"
)

// Principal captures the caller identity and roles/groups.
type Principal struct {
    Subject string
    Roles   []string
    Groups  []string
}

// Can determines whether a principal is authorized to perform an action on a given state key.
// Stub: allow all for now to avoid regressions until RBAC policy is wired.
func Can(_ Principal, _ Action, _ string) bool {
    return true
}

