package horizon

import (
	"net/http"
)

// requestCacheHeadersMiddleware adds caching headers to each response.
func requestCacheHeadersMiddleware(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Before changing this read Stack Overflow answer about staled request
		// in older versions of Chrome:
		// https://stackoverflow.com/questions/27513994/chrome-stalls-when-making-multiple-requests-to-same-resource
		w.Header().Set("Cache-Control", "no-cache, no-store, max-age=0")
		h.ServeHTTP(w, r)
	})
}
