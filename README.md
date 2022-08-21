[![Build Status](https://github.com/azazeal/singleflight/actions/workflows/build.yml/badge.svg)](https://github.com/azazeal/singleflight/actions/workflows/build.yml)
[![Coverage Report](https://coveralls.io/repos/github/azazeal/singleflight/badge.svg?branch=master)](https://coveralls.io/github/azazeal/singleflight?branch=master)
[![Go Reference](https://pkg.go.dev/badge/github.com/azazeal/singleflight.svg)](https://pkg.go.dev/github.com/azazeal/singleflight)

# singleflight

Package singleflight implements a call sharing mechanism.

## Example usage

```go
// This package demonstrates how an implementation of a HTTP server might use the singleflight
// package in order to minimize roundtrips to its database.
package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/azazeal/singleflight"
)

func main() {
	http.HandleFunc("/", handler)

	if err := http.ListenAndServe(":8080", nil); !errors.Is(err, http.ErrServerClosed) {
		log.Fatal(err)
	}
}

func handler(w http.ResponseWriter, r *http.Request) {
	name := r.URL.Query().Get("name")

	switch id, err := fetchCustomerID(r.Context(), name); {
	case err == nil:
		fmt.Fprintf(w, "%d", id)
	case errors.Is(err, errNotFound):
		renderCode(w, http.StatusNotFound)
	default:
		renderCode(w, http.StatusInternalServerError)
	}
}

// fetchCustomerID returns the id of the named customer.
//
// Concurrent callers of fetchCustomerID will share the result.
func fetchCustomerID(ctx context.Context, name string) (int64, error) {
	return caller.Call(ctx, name, doFetchCustomerID)
}

var (
	errNotFound = errors.New("not found")

	// caller is used by fetchCustomerID to reduce the number of calls to doFetchCustomerID.
	caller singleflight.Caller[string, int64]
)

func doFetchCustomerID(ctx context.Context) (id int64, err error) {
	time.Sleep(time.Millisecond << 7)

	if name := caller.KeyFromContext(ctx); name == "customer-1" {
		id = 1
	} else {
		err = errNotFound
	}

	return
}

func renderCode(w http.ResponseWriter, code int) {
	http.Error(w, http.StatusText(code), code)
}
```
