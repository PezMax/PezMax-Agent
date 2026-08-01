package backend

import "context"

type authTokenKey struct{}

func WithAuthToken(ctx context.Context, token string) context.Context {
	if token == "" {
		return ctx
	}
	return context.WithValue(ctx, authTokenKey{}, token)
}

func authTokenFromContext(ctx context.Context) string {
	value, _ := ctx.Value(authTokenKey{}).(string)
	return value
}
