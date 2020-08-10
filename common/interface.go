package common

import "fmt"

// WalkFunc is called at each item visited by Walk.
// If an error is returned, the walk may continue
// if the Walker is configured to continue on error.
// The sole exception is the error value ErrStopWalk,
// which stops the walk without an actual error.
type WalkFunc func(f File) error

// ErrStopWalk signals Walk to break without error.
var ErrStopWalk = fmt.Errorf("walk stopped")
