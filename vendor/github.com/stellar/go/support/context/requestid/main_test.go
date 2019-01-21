package requestid

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/go-chi/chi/middleware"
)

func TestRequestContext(t *testing.T) {
	ctx := Context(context.Background(), "2")
	assert.Equal(t, "2", ctx.Value(&key))

	ctx2 := Context(ctx, "3")
	assert.Equal(t,"3", ctx2.Value(&key))
	assert.Equal(t,"2", ctx.Value(&key))
}

func TestRequestContextFromCHI(t *testing.T) {
	ctx := context.WithValue(context.Background(), middleware.RequestIDKey, "foobar")
	ctx2 := ContextFromChi(ctx)
	assert.Equal(t, "foobar", FromContext(ctx2))
}

func TestRequestFromContext(t *testing.T) {
	ctx := Context(context.Background(), "2")
	ctx2 := Context(ctx, "3")
	assert.Equal(t, "", FromContext(context.Background()))
	assert.Equal(t, "2", FromContext(ctx))
	assert.Equal(t, "3", FromContext(ctx2))
}
