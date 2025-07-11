package main

import (
	"context"
	"fmt"

	"github.com/maypok86/otter/v2"
)

func main() {
	cache := otter.Must(&otter.Options[string, any]{
		MaximumSize: 10000,
	})

	v, err := cache.Get(context.Background(), "123", otter.LoaderFunc[string, any](func(ctx context.Context, key string) (any, error) {
		return nil, otter.ErrNotFound
	}))
	fmt.Println(v, err)

	v, ok := cache.Set("123", 123)

	fmt.Println(v, ok)
	v, err = cache.Get(context.Background(), "123", nil)
	fmt.Println(v, err)

	cache.BulkGet()
}
