// Package requestid provides functions to support embedded and retrieving
// a request id from a go context tree
package requestid

import (
	"context"

	"github.com/go-chi/chi/middleware"
)

var key = 0

// Context create a context from the provided parent and the provided request id
// string.
func Context(ctx context.Context, reqid string) context.Context {
	return context.WithValue(ctx, &key, reqid)
}

// ContextFromChi returns a new context bound with the value of the request id.
func ContextFromChi(ctx context.Context) context.Context {
	reqid := middleware.GetReqID(ctx)
	return Context(ctx, reqid)
}

// FromContext returns the set request id, if one has been set, from the
// provided context returns "" if no requestid has been set
//
func FromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}

	val := ctx.Value(&key)
	if val == nil {
		return ""
	}

	result, ok := val.(string)

	if ok {
		return result
	}

	return ""
}
