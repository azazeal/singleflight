[![Build Status](https://github.com/azazeal/singleflight/actions/workflows/build.yml/badge.svg)](https://github.com/azazeal/singleflight/actions/workflows/build.yml)
[![Coverage Report](https://coveralls.io/repos/github/azazeal/singleflight/badge.svg?branch=master)](https://coveralls.io/github/azazeal/singleflight?branch=master)
[![Go Reference](https://pkg.go.dev/badge/github.com/azazeal/singleflight.svg)](https://pkg.go.dev/github.com/azazeal/singleflight)

# singleflight

Package singleflight implements a call sharing mechanism: while a call for a
key is in flight, further calls for the same key wait for it and share its
result instead of starting one of their own.

## Example

```go
package main

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/azazeal/singleflight"
)

func main() {
	var wg sync.WaitGroup

	for range 3 {
		wg.Go(func() {
			v, err := lookups.Call(context.Background(), "world", lookup)
			fmt.Println(v, err)
		})
	}

	wg.Wait()
}

var lookups singleflight.Caller[string, string]

// lookup stands in for a slow query for the name that ctx carries.
func lookup(ctx context.Context) (string, error) {
	name := lookups.KeyFromContext(ctx)
	fmt.Println("looking up:", name)

	time.Sleep(100 * time.Millisecond)

	return "hello, " + name, nil
}
```

The three calls overlap, so `lookup` runs once and all of them get its result:

```
looking up: world
hello, world <nil>
hello, world <nil>
hello, world <nil>
```
