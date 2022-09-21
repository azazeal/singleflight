// Package singleflight implements a call sharing mechanism.
package singleflight

import (
	"context"
	"sync"

	"golang.org/x/sync/semaphore"
)

// Caller wraps the functionality of the call sharing mechanism.
//
// A Caller must not be copied after first use.
type Caller[K comparable, V any] struct {
	mu    sync.Mutex
	calls map[K]*call[V]
}

const (
	readerWeight = 1 << (30 * iota)
	writerWeight
)

type call[V any] struct {
	sem *semaphore.Weighted
	val V
	err error
}

// Call calls fn and returns the results. Concurrent callers sharing a key will also share the results of the first
// call.
//
// fn may access the key passed to Call via KeyFromContext.
func (caller *Caller[K, V]) Call(ctx context.Context, key K, fn func(context.Context) (V, error)) (V, error) {
	caller.mu.Lock()

	if caller.calls == nil {
		caller.calls = make(map[K]*call[V])
	}

	// check whether an in-flight call exists for the key
	if inflight, ok := caller.calls[key]; ok {
		// an in-flight call exists; attach to it as a reader and return its result once available
		caller.mu.Unlock()

		if err := inflight.sem.Acquire(ctx, readerWeight); err != nil {
			var zero V
			return zero, err
		}
		defer inflight.sem.Release(readerWeight)

		return inflight.val, inflight.err
	}

	// there's no in-flight call; start one
	call := &call[V]{
		sem: semaphore.NewWeighted(writerWeight),
	}
	_ = call.sem.Acquire(context.Background(), writerWeight) // guaranteed to succeed

	caller.calls[key] = call
	caller.mu.Unlock()

	call.val, call.err = fn(context.WithValue(ctx, contextKeyType[K]{}, key))

	// the call has finished; we're still the only active caller so we can mark
	// this call as no longer taking place by deleting it from the map
	caller.mu.Lock()
	call.sem.Release(writerWeight)
	delete(caller.calls, key)
	caller.mu.Unlock()

	return call.val, call.err
}

type contextKeyType[K comparable] struct{}

// KeyFromContext returns the key ctx carries. It panics in case ctx carries no key.
func (*Caller[K, V]) KeyFromContext(ctx context.Context) K {
	return ctx.Value(contextKeyType[K]{}).(K)
}
