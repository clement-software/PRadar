package sqlite

import (
	"context"
	"fmt"
)

// MakeDue brings every queued analysis forward to now. Waiting for a real
// anti-rebond window would make a test sleep for minutes; the durable
// behaviour under test is what happens once work is due.
func (s *Store) MakeDue(ctx context.Context) error {
	if _, err := s.db.ExecContext(ctx, `UPDATE analysis_jobs SET available_unix = ? WHERE status = 'queued'`, s.now().Unix()); err != nil {
		return fmt.Errorf("make queued work due: %w", err)
	}
	return nil
}
