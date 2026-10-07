// Package sessionctx переносит сессию вошедшего пользователя через context запроса:
// её кладёт middleware проверки сессии, читают хендлеры.
package sessionctx

import (
	"context"

	"ldap-admin/internal/domain/session"
)

type contextKey struct{}

func With(ctx context.Context, current session.Session) context.Context {
	return context.WithValue(ctx, contextKey{}, current)
}

func From(ctx context.Context) (session.Session, bool) {
	current, found := ctx.Value(contextKey{}).(session.Session)
	return current, found
}
