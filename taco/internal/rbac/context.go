package rbac

import "context"

type contextKey string

const (
	principalKey contextKey = "principal"
)

// ContextWithPrincipal adds a principal to the context
func ContextWithPrincipal(ctx context.Context, principal Principal) context.Context {
	return context.WithValue(ctx, principalKey, principal)
}

// PrincipalFromContext retrieves the principal from the context
func PrincipalFromContext(ctx context.Context) (Principal, bool) {
	principal, ok := ctx.Value(principalKey).(Principal)
	return principal, ok
}

