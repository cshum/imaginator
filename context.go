package imagor

import (
	"context"
	"errors"
	"sync"
)

type imagorContextKey struct{}

type imagorContextRef struct {
	funcs []func()
	l     sync.Mutex
	Cache sync.Map
}

func (r *imagorContextRef) Defer(fn func()) {
	r.l.Lock()
	r.funcs = append(r.funcs, fn)
	r.l.Unlock()
}

func (r *imagorContextRef) Call() {
	r.l.Lock()
	for _, fn := range r.funcs {
		fn()
	}
	r.funcs = nil
	r.l.Unlock()
}

// WithContext context with imagor defer handling and cache
func WithContext(ctx context.Context) context.Context {
	r := &imagorContextRef{}
	ctx = context.WithValue(ctx, imagorContextKey{}, r)
	go func() {
		<-ctx.Done()
		r.Call()
	}()
	return ctx
}

func mustContextValue(ctx context.Context) *imagorContextRef {
	if r, ok := ctx.Value(imagorContextKey{}).(*imagorContextRef); ok && r != nil {
		return r
	}
	panic(errors.New("not imagor context"))
}

// Defer add func to context, defer called at the end of request
func Defer(ctx context.Context, fn func()) {
	mustContextValue(ctx).Defer(fn)
}

// ContextCachePut put cache within the imagor request context lifetime
func ContextCachePut(ctx context.Context, key any, val any) {
	if r, ok := ctx.Value(imagorContextKey{}).(*imagorContextRef); ok && r != nil {
		r.Cache.Store(key, val)
	}
}

// ContextCacheGet get cache within the imagor request context lifetime
func ContextCacheGet(ctx context.Context, key any) (any, bool) {
	if r, ok := ctx.Value(imagorContextKey{}).(*imagorContextRef); ok && r != nil {
		return r.Cache.Load(key)
	}
	return nil, false
}
