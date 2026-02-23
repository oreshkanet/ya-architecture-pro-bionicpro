package auth

import "context"

type userIDKey struct{}

func WithUserID(ctx context.Context, userID string) context.Context {
	return context.WithValue(ctx, userIDKey{}, userID)
}

func UserIDFromContext(ctx context.Context) string {
	v, _ := ctx.Value(userIDKey{}).(string)
	return v
}
