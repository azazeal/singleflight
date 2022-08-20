package singleflight

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func Test(t *testing.T) {
	const key = "key"

	var (
		caller     Caller[string, bool]
		executions int64
	)

	fn := func(ctx context.Context) (bool, error) {
		time.Sleep(time.Second >> 1)

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
	assert.Same(t, err1, assert.AnError)

	assert.True(t, r2)
	assert.Same(t, err2, assert.AnError)

	assert.Equal(t, int64(1), executions)
}
