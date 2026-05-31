//go:build !windows

package platform

// No single-instance lock on non-Windows platforms yet.

// AcquireSingleInstance always allows execution and returns an idle channel.
func AcquireSingleInstance() (bool, <-chan struct{}) {
	ch := make(chan struct{})
	return true, ch
}

// PingExistingInstance is a no-op.
func PingExistingInstance() {}
