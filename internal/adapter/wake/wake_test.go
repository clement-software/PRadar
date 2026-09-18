package wake_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/clement-software/PRadar/internal/adapter/wake"
)

// jumpingClock returns a wall clock the test moves forward by hand, which is
// what a sleeping machine looks like from inside the process.
type jumpingClock struct {
	mu  sync.Mutex
	now time.Time
}

func (c *jumpingClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *jumpingClock) jump(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(d)
}

func TestDetector_ReportsOneWakePerSleep(t *testing.T) {
	t.Parallel()
	clock := &jumpingClock{now: time.Date(2026, 9, 18, 8, 0, 0, 0, time.UTC)}
	detector := &wake.Detector{Check: 5 * time.Millisecond, Tolerance: 10 * time.Millisecond, Now: clock.Now}
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	done := make(chan struct{})
	go func() { defer close(done); detector.Run(ctx) }()

	// Ordinary sampling advances the clock like the ticker does.
	for range 4 {
		clock.jump(5 * time.Millisecond)
		time.Sleep(6 * time.Millisecond)
	}
	select {
	case at := <-detector.Wakes():
		t.Fatalf("a running machine must not look asleep: %s", at)
	default:
	}

	clock.jump(2 * time.Hour) // the machine slept
	select {
	case <-detector.Wakes():
	case <-time.After(2 * time.Second):
		t.Fatal("a wall-clock jump must report a wake")
	}
	// The resume is reported once, not once per sample.
	clock.jump(5 * time.Millisecond)
	time.Sleep(20 * time.Millisecond)
	select {
	case at := <-detector.Wakes():
		t.Fatalf("a single sleep must report a single wake, got another at %s", at)
	default:
	}
	cancel()
	<-done
}
