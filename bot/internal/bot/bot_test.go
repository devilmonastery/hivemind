package bot

import (
	"errors"
	"log/slog"
	"testing"
	"time"
)

func TestOpenDiscordConnectionRetriesUntilSuccess(t *testing.T) {
	attempts := 0
	var delays []time.Duration

	err := openDiscordConnection(func() error {
		attempts++
		if attempts < 3 {
			return errors.New("Discord unavailable")
		}
		return nil
	}, slog.Default(), func(delay time.Duration) {
		delays = append(delays, delay)
	})

	if err != nil {
		t.Fatalf("openDiscordConnection() error = %v", err)
	}
	if attempts != 3 {
		t.Fatalf("attempts = %d, want 3", attempts)
	}
	if len(delays) != 2 || delays[0] != 2*time.Second || delays[1] != 4*time.Second {
		t.Fatalf("delays = %v, want [2s 4s]", delays)
	}
}

func TestOpenDiscordConnectionReturnsErrorAfterRetries(t *testing.T) {
	wantErr := errors.New("Discord unavailable")
	attempts := 0

	err := openDiscordConnection(func() error {
		attempts++
		return wantErr
	}, slog.Default(), func(time.Duration) {})

	if !errors.Is(err, wantErr) {
		t.Fatalf("error = %v, want wrapped connection error", err)
	}
	if attempts != discordConnectionAttempts {
		t.Fatalf("attempts = %d, want %d", attempts, discordConnectionAttempts)
	}
}
