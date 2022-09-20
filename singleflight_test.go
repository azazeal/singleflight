package singleflight

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

const (
	longPause = time.Second >> (1 + iota)
	mediumPause
	shortPause
)

func Test(t *testing.T) {
	t.Parallel()

	const key = "key"

	var (
		caller     Caller[string, bool]
		executions int64
	)

	fn := func(ctx context.Context) (bool, error) {
		time.Sleep(shortPause)

		_ = atomic.AddInt64(&executions, 1)

		return caller.KeyFromContext(ctx) == key, assert.AnError
	}

	var wg sync.WaitGroup

	doCall := func(r *bool, err *error) {
		wg.Add(1)

		go func() {
			defer wg.Done()

			*r, *err = caller.Call(context.Background(), key, fn)
		}()
	}

	// start the first caller
	var r1 bool
	var err1 error
	doCall(&r1, &err1)

	// start the second caller
	var r2 bool
	var err2 error
	doCall(&r2, &err2)

	wg.Wait()

	assert.True(t, r1)
	assert.Same(t, assert.AnError, err1)

	assert.True(t, r2)
	assert.Same(t, assert.AnError, err2)

	assert.Equal(t, int64(1), executions)

	// ensure further executions once concurrent callers finish
	r3, err3 := caller.Call(context.Background(), key+"1", fn)

	assert.False(t, r3)
	assert.Same(t, assert.AnError, err3)
	assert.Equal(t, int64(2), executions)
}

func TestSecondaryContextCancellation(t *testing.T) {
	t.Parallel()

	fn := func(ctx context.Context) (bool, error) {
		time.Sleep(longPause)

		return true, nil
	}

	const key = "key"
	var (
		caller           Caller[string, bool]
		got1, got2, got3 bool
		err1, err2, err3 error
		wg               sync.WaitGroup
	)

	wg.Add(1)
	go func() {
		defer wg.Done()

		got1, err1 = caller.Call(context.Background(), key, fn)
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()

		time.Sleep(shortPause)

		ctx, cancel := context.WithCancel(context.Background())
		go func() {
			time.Sleep(shortPause)
			cancel()
		}()

		got2, err2 = caller.Call(ctx, key, fn)
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()

		time.Sleep(shortPause)

		ctx, cancel := context.WithTimeout(context.Background(), mediumPause)
		defer cancel()

		got3, err3 = caller.Call(ctx, key, fn)
	}()

	wg.Wait()

	assert.True(t, got1)
	assert.NoError(t, err1)
	assert.False(t, got2)
	assert.ErrorIs(t, err2, context.Canceled)
	assert.False(t, got3)
	assert.ErrorIs(t, err3, context.DeadlineExceeded)
}
