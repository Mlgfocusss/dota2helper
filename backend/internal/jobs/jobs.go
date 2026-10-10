// Package jobs запускает периодические фоновые задачи в горутинах.
package jobs

import (
	"context"
	"fmt"
	"log/slog"
	"runtime/debug"
	"sync"
	"time"
)

type Job struct {
	Name string
	// Interval — пауза между успешными запусками
	Interval time.Duration
	// RetryInterval — пауза после ошибки (по умолчанию Interval)
	RetryInterval time.Duration
	// Delay — задержка перед первым запуском
	Delay time.Duration
	Run   func(ctx context.Context) error
}

// Start запускает задачи и возвращает функцию, которая ждёт их завершения
// после отмены ctx.
func Start(ctx context.Context, logger *slog.Logger, jobs ...Job) (wait func()) {
	var wg sync.WaitGroup
	for _, j := range jobs {
		wg.Add(1)
		go func() {
			defer wg.Done()
			loop(ctx, logger.With("job", j.Name), j)
		}()
	}
	return wg.Wait
}

func loop(ctx context.Context, logger *slog.Logger, j Job) {
	retry := j.RetryInterval
	if retry <= 0 {
		retry = j.Interval
	}
	next := j.Delay
	for {
		timer := time.NewTimer(next)
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}

		start := time.Now()
		if err := runSafe(ctx, j.Run); err != nil {
			if ctx.Err() != nil {
				return
			}
			logger.Warn("job failed", "err", err, "dur", time.Since(start), "retry_in", retry)
			next = retry
			continue
		}
		logger.Debug("job done", "dur", time.Since(start))
		next = j.Interval
	}
}

// runSafe не даёт панике в задаче уронить весь сервер.
func runSafe(ctx context.Context, run func(context.Context) error) (err error) {
	defer func() {
		if rec := recover(); rec != nil {
			err = fmt.Errorf("panic: %v\n%s", rec, debug.Stack())
		}
	}()
	return run(ctx)
}
