package singleflight

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
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

		return caller.KeyFromContext(ctx) == key, errAssert
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

	assertTrue(t, r1)
	assertError(t, err1)

	assertTrue(t, r2)
	assertError(t, err2)
	assertEqual(t, executions, 1)

	// ensure further executions once concurrent callers finish
	r3, err3 := caller.Call(context.Background(), key+"1", fn)

	assertFalse(t, r3)
	assertError(t, err3)
	assertEqual(t, executions, 2)
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

	assertTrue(t, got1)
	assertNil(t, err1)
	assertFalse(t, got2)
	assertErrorIs(t, err2, context.Canceled)
	assertFalse(t, got3)
	assertErrorIs(t, err3, context.DeadlineExceeded)
}

func assertEqual[T comparable](t *testing.T, actual, expected T) {
	t.Helper()

	if actual == expected {
		return
	}

	t.Errorf("expected %v, got: %v", expected, actual)
}

func assertNil(t *testing.T, err error) {
	t.Helper()

	if err == nil {
		return
	}

	t.Errorf("expected nil, got %v", err)
}

var errAssert = errors.New("assert error")

func assertError(t *testing.T, actual error) {
	t.Helper()

	if errAssert == actual { //nolint:errorlint
		return
	}

	t.Errorf("expected %q, got %q", errAssert, actual)
}

func assertErrorIs(t *testing.T, actual, target error) {
	t.Helper()

	if errors.Is(actual, target) {
		return
	}

	t.Errorf("expected the error to be wrapping %q", target)
}

func assertTrue(t *testing.T, actual bool) {
	t.Helper()

	if actual {
		return
	}

	t.Error("expected true")
}

func assertFalse(t *testing.T, actual bool) {
	t.Helper()

	if !actual {
		return
	}

	t.Error("expected false")
}
