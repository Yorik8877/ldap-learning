package auth

import (
	"context"
	"net/http"

	"samba-admin/internal/domain/session"
)

const sessionCookieName = "session"

// HttpOnly — JavaScript страницы не прочитает cookie, XSS её не украдёт.
// SameSite=Strict — браузер не отправит cookie с запросом, начатым на чужом сайте (защита от CSRF).
// Secure не ставится: бэкенд слушает обычный HTTP. За HTTPS-прокси его нужно включить.
func setSessionCookie(w http.ResponseWriter, opened session.Session) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    opened.ID,
		Path:     "/",
		Expires:  opened.ExpiresAt,
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
	})
}

// clearSessionCookie просит браузер удалить cookie: MaxAge < 0 даёт Max-Age=0.
func clearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
	})
}

// sessionContextKey — свой тип ключа, чтобы ключ не совпал с ключом другого пакета.
type sessionContextKey struct{}

func contextWithSession(parent context.Context, current session.Session) context.Context {
	return context.WithValue(parent, sessionContextKey{}, current)
}

func sessionFromContext(ctx context.Context) (session.Session, bool) {
	current, found := ctx.Value(sessionContextKey{}).(session.Session)
	return current, found
}
