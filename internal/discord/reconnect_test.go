package discord

import (
	"testing"
	"time"
)

func TestBackoffSequence(t *testing.T) {
	cases := []struct {
		attempt int
		want    time.Duration
	}{
		{-3, 1 * time.Second},
		{0, 1 * time.Second},
		{1, 2 * time.Second},
		{2, 4 * time.Second},
		{3, 8 * time.Second},
		{4, 16 * time.Second},
		{5, 30 * time.Second},
		{6, 30 * time.Second},
		{100, 30 * time.Second},
	}
	for _, tc := range cases {
		if got := backoff(tc.attempt); got != tc.want {
			t.Errorf("backoff(%d) = %v, want %v", tc.attempt, got, tc.want)
		}
	}
}
