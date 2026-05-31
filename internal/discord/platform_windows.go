//go:build windows

package discord

import (
	"fmt"
	"io"
	"os"
)

// dial opens the first available discord-ipc-{0..9} named pipe.
// Exposed as a var so tests can inject a fake transport.
var dial = func() (io.ReadWriteCloser, error) {
	var lastErr error
	for i := 0; i < 10; i++ {
		path := fmt.Sprintf(`\\.\pipe\discord-ipc-%d`, i)
		f, err := os.OpenFile(path, os.O_RDWR, 0)
		if err == nil {
			return f, nil
		}
		lastErr = err
	}
	if lastErr != nil {
		return nil, fmt.Errorf("%w: %v", ErrPipeNotFound, lastErr)
	}
	return nil, ErrPipeNotFound
}
