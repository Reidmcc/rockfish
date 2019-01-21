package horizon

import (
	"net/http"
)

func (web *Web) RateLimitMiddleware(next http.Handler) http.Handler {
	return web.rateLimiter.Throttle(next)
}
