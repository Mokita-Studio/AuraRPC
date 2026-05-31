//go:build linux || darwin

package discord

import (
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
)

// dial opens the first available discord-ipc-{0..9} Unix socket under
// $XDG_RUNTIME_DIR, $TMPDIR or /tmp. Exposed as a var for test injection.
var dial = func() (io.ReadWriteCloser, error) {
	var dirs []string
	if d := os.Getenv("XDG_RUNTIME_DIR"); d != "" {
		dirs = append(dirs, d)
	}
	if d := os.Getenv("TMPDIR"); d != "" {
		dirs = append(dirs, d)
	}
	dirs = append(dirs, "/tmp")

	var lastErr error
	for _, dir := range dirs {
		for i := 0; i < 10; i++ {
			path := filepath.Join(dir, fmt.Sprintf("discord-ipc-%d", i))
			c, err := net.Dial("unix", path)
			if err == nil {
				return c, nil
			}
			lastErr = err
		}
	}
	if lastErr != nil {
		return nil, fmt.Errorf("%w: %v", ErrPipeNotFound, lastErr)
	}
	return nil, ErrPipeNotFound
}
