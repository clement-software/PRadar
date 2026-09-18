// Package wake reports when the machine resumed from sleep, without a
// platform framework: it watches the gap between wall time and the time a
// short ticker should have taken.
package wake

import (
	"context"
	"sync"
	"time"
)

// DefaultCheck is how often the detector samples the clock.
const DefaultCheck = 30 * time.Second

// Detector emits one value per resume. A resume is a wall-clock jump much
// larger than the sampling interval, which is what a sleeping machine does.
type Detector struct {
	// Check is the sampling interval; zero means DefaultCheck.
	Check time.Duration
	// Tolerance is how much lateness is normal before a gap counts as sleep;
	// zero means four times Check.
	Tolerance time.Duration
	// Now is the wall clock; zero means time.Now.
	Now func() time.Time

	once  sync.Once
	wakes chan time.Time
}

// Wakes is the channel the collector listens on. One buffered slot is enough:
// a pending resume already means one catch-up reconciliation.
func (d *Detector) Wakes() <-chan time.Time {
	d.once.Do(func() { d.wakes = make(chan time.Time, 1) })
	return d.wakes
}

// Run samples the clock until ctx ends. It owns no goroutine of its own: the
// caller decides where it runs.
func (d *Detector) Run(ctx context.Context) {
	check := max(d.Check, 0)
	if check == 0 {
		check = DefaultCheck
	}
	tolerance := d.Tolerance
	if tolerance == 0 {
		tolerance = 4 * check
	}
	now := d.Now
	if now == nil {
		now = time.Now
	}
	_ = d.Wakes() // create the channel before the first send
	ticker := time.NewTicker(check)
	defer ticker.Stop()
	last := now()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
		current := now()
		if current.Sub(last) > check+tolerance {
			// A full channel already carries a pending resume; one catch-up
			// reconciliation is enough.
			select {
			case d.wakes <- current:
			default:
			}
		}
		last = current
	}
}
