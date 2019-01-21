package horizon

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi"
	"github.com/go-chi/chi/middleware"
	chimiddleware "github.com/go-chi/chi/middleware"
	"github.com/stellar/go/services/horizon/internal/log"
	"github.com/stellar/go/services/horizon/internal/render"
)

// LoggerMiddleware is the middleware that logs http requests and resposnes
// to the logging subsytem of horizon.
func LoggerMiddleware(h http.Handler) http.Handler {
	fn := func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		mw := middleware.NewWrapResponseWriter(w, r.ProtoMajor)

		logger := log.WithField("req", chimiddleware.GetReqID(ctx))

		ctx = log.Set(ctx, logger)

		logStartOfRequest(ctx, r)

		then := time.Now()
		h.ServeHTTP(mw, r.WithContext(ctx))
		duration := time.Now().Sub(then)
		// Checking `Accept` header from user request because if the streaming connection
		// is reset before sending the first event no Content-Type header is sent in a response.
		acceptHeader := r.Header.Get("Accept")
		streaming := strings.Contains(acceptHeader, render.MimeEventStream)
		logEndOfRequest(ctx, r, duration, mw, streaming)
	}

	return http.HandlerFunc(fn)
}

func logStartOfRequest(ctx context.Context, r *http.Request) {
	log.Ctx(ctx).WithFields(log.F{
		"path":         r.URL.String(),
		"method":       r.Method,
		"ip":           remoteAddrIP(r),
		"ip_port":      r.RemoteAddr,
		"forwarded_ip": firstXForwardedFor(r),
		"host":         r.Host,
	}).Info("Starting request")
}

func logEndOfRequest(ctx context.Context, r *http.Request, duration time.Duration, mw middleware.WrapResponseWriter, streaming bool) {
	log.Ctx(ctx).WithFields(log.F{
		"path":         r.URL.String(),
		"route":        chi.RouteContext(r.Context()).RoutePattern(),
		"method":       r.Method,
		"ip":           remoteAddrIP(r),
		"ip_port":      r.RemoteAddr,
		"forwarded_ip": firstXForwardedFor(r),
		"host":         r.Host,
		"status":       mw.Status(),
		"bytes":        mw.BytesWritten(),
		"duration":     duration.Seconds(),
		"streaming":    streaming,
	}).Info("Finished request")
}
