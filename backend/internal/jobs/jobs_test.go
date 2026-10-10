package jobs

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"sync/atomic"
	"testing"
	"time"
)

func TestJobRunsRetriesAndStops(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	ctx, cancel := context.WithCancel(context.Background())

	var calls atomic.Int32
	wait := Start(ctx, logger, Job{
		Name:          "test",
		Interval:      time.Hour,
		RetryInterval: 10 * time.Millisecond,
		Run: func(context.Context) error {
			switch calls.Add(1) {
			case 1:
				return errors.New("first run fails")
			case 2:
				panic("second run panics")
			}
			return nil
		},
	})

	deadline := time.Now().Add(2 * time.Second)
	for calls.Load() < 3 && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	if calls.Load() != 3 {
		t.Fatalf("после ошибки и паники задача должна повториться: запусков %d", calls.Load())
	}

	cancel()
	done := make(chan struct{})
	go func() {
		wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("задача не остановилась после отмены контекста")
	}
}
