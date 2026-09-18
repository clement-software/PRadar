//go:build !darwin

package desktop

import "context"

// Show reports that this platform has no native window. The interface stays
// reachable in a browser at the address the application prints.
func Show(context.Context, Window) error { return ErrUnsupported }
