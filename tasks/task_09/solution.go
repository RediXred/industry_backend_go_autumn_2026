package main

import (
	"context"
	"errors"
	"sync"
)

type Result[R any] struct {
	Value R
	Err   error
}

var ErrInvalidWorkers = errors.New("workers must be positive")

func ParallelMap[T any, R any](ctx context.Context, workers int, in []T, fn func(context.Context, T) (R, error)) ([]Result[R], error) {
	if workers <= 0 {
		return nil, ErrInvalidWorkers
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if len(in) == 0 {
		return []Result[R]{}, nil
	}

	tasks := make(chan int)
	rez := make([]Result[R], len(in))

	cnt := min(len(in), workers)

	var wg sync.WaitGroup
	for range cnt {
		wg.Go(func() {
			for idx := range tasks {
				if ctx.Err() != nil {
					return
				}

				v, err := fn(ctx, in[idx])
				if err != nil {
					rez[idx] = Result[R]{Err: err}
				} else {
					rez[idx] = Result[R]{v, nil}
				}
			}
		})
	}

	for idx := range in {
		if ctx.Err() != nil {
			break
		}
		select {
		case tasks <- idx:
		case <-ctx.Done():
			continue
		}
	}
	close(tasks)
	wg.Wait()

	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return rez, nil
}
