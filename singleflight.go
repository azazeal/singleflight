// Package singleflight implements a call sharing mechanism.
package singleflight

import (
	"context"
	"sync"
)

// Caller implements a call-sharing mechanism. It must not be copied
// after first use.
type Caller[K comparable, V any] struct {
	calls map[K]*call[V]
	mu    sync.Mutex // protects calls

	// NOTE(@azazeal): it's atypical to have calls declared
	// before mu in this struct, but this reduces the struct's
	// pointer size to 8 from 16.
}

type call[V any] struct {
	// done is made by the first reader and closed once the fields below are
	// set; it stays nil when no reader shows up. It is only read and written
	// under the Caller's mutex.
	done chan struct{}

	val   V
	err   error
	panik any // the value fn panicked with, if it did
}

// Call runs fn and returns its results.
//
// While a call for a key is in flight, further calls for it wait for that call
// and return its results instead of running fn. If ctx is done before the
// results are available, Call returns ctx.Err().
//
// If fn panics, every caller sharing the call panics with the same value.
//
// fn may retrieve the key via [Caller.KeyFromContext].
func (caller *Caller[K, V]) Call(ctx context.Context, key K, fn func(context.Context) (V, error)) (V, error) {
	caller.mu.Lock()

	// check whether an in-flight call exists for the key
	if inflight, ok := caller.calls[key]; ok {
		// an in-flight call exists; attach to it as a reader and return
		// its result once available
		if inflight.done == nil {
			inflight.done = make(chan struct{})
		}
		done := inflight.done
		caller.mu.Unlock()

		select {
		case <-done:
		case <-ctx.Done():
			// ctx lost the race or both were ready; take the result if
			// it's there
			select {
			case <-done:
			default:
				var zero V
				return zero, ctx.Err()
			}
		}

		if inflight.panik != nil {
			panic(inflight.panik)
		}

		return inflight.val, inflight.err
	}

	// there's no in-flight call; start one
	v := &call[V]{}

	if caller.calls == nil {
		caller.calls = map[K]*call[V]{
			key: v,
		}
	} else {
		caller.calls[key] = v
	}
	caller.mu.Unlock()

	defer func() {
		// if fn panicked, keep the value so that the readers can re-raise it.
		// It must be stored before done is closed, since that is what
		// publishes it to them.
		v.panik = recover()

		// the call has finished; drop it from the map so that later callers
		// start a new one, then wake the readers, if any. The channel is read
		// after the delete, so no reader can make one that we would miss.
		caller.mu.Lock()
		delete(caller.calls, key)
		done := v.done
		caller.mu.Unlock()
		if done != nil {
			close(done)
		}

		if v.panik != nil {
			panic(v.panik)
		}
	}()

	v.val, v.err = fn(context.WithValue(ctx, contextKeyType[K]{}, key))

	return v.val, v.err
}

type contextKeyType[K comparable] struct{}

// MaybeKeyFromContext returns the key ctx carries, which is the key given to
// the [Caller.Call] that ctx descends from. The boolean reports whether ctx
// carries a key at all.
func (*Caller[K, V]) MaybeKeyFromContext(ctx context.Context) (K, bool) {
	k, ok := ctx.Value(contextKeyType[K]{}).(K)
	return k, ok
}

// KeyFromContext is like [Caller.MaybeKeyFromContext], but panics if ctx
// carries no key.
func (*Caller[K, V]) KeyFromContext(ctx context.Context) K {
	return ctx.Value(contextKeyType[K]{}).(K)
}
