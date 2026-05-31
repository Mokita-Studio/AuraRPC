package discord

import "time"

// backoff returns the wait before the next reconnect attempt:
// 1s, 2s, 4s, 8s, 16s, then capped at 30s.
func backoff(attempt int) time.Duration {
	if attempt < 0 {
		attempt = 0
	}
	if attempt >= 5 {
		return 30 * time.Second
	}
	return time.Duration(1<<uint(attempt)) * time.Second
}
