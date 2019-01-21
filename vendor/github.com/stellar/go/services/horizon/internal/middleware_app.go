package horizon

import (
	"net/http"
)

// Adds the "app" context into every request, so that subsequence middleware
// or handlers can retrieve a horizon.App instance
func (app *App) Middleware(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := app.Context(r.Context())
		h.ServeHTTP(w, r.WithContext(ctx))
	})
}
