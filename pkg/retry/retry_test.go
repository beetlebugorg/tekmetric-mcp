package retry

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"
)

// tempErr reports a fixed value from Temporary().
type tempErr struct {
	msg  string
	temp bool
}

func (e *tempErr) Error() string   { return e.msg }
func (e *tempErr) Temporary() bool { return e.temp }

// newFast builds a Retryer whose backoff is always zero, so tests do not sleep.
func newFast(maxRetries int) *Retryer {
	return New(maxRetries, 0)
}

func TestDoSucceedsOnFirstAttempt(t *testing.T) {
	calls := 0
	err := newFast(3).Do(t.Context(), func() error {
		calls++
		return nil
	})

	if err != nil {
		t.Errorf("Do() error = %v, want nil", err)
	}
	if calls != 1 {
		t.Errorf("calls = %d, want 1", calls)
	}
}

func TestDoRetriesTemporaryErrorUntilSuccess(t *testing.T) {
	calls := 0
	err := newFast(3).Do(t.Context(), func() error {
		calls++
		if calls < 3 {
			return &tempErr{msg: "temporary error with status 503", temp: true}
		}
		return nil
	})

	if err != nil {
		t.Errorf("Do() error = %v, want nil", err)
	}
	if calls != 3 {
		t.Errorf("calls = %d, want 3", calls)
	}
}

func TestDoStopsAfterMaxRetries(t *testing.T) {
	tests := []struct {
		name       string
		maxRetries int
		wantCalls  int
	}{
		{"no retries", 0, 1},
		{"one retry", 1, 2},
		{"three retries", 3, 4},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			calls := 0
			err := newFast(tt.maxRetries).Do(t.Context(), func() error {
				calls++
				return &tempErr{msg: "temporary error with status 500", temp: true}
			})

			if err == nil {
				t.Fatal("Do() returned nil, want an error")
			}
			if calls != tt.wantCalls {
				t.Errorf("calls = %d, want %d", calls, tt.wantCalls)
			}
		})
	}
}

func TestDoDoesNotRetryPermanentError(t *testing.T) {
	calls := 0
	want := errors.New("API request failed with status 404")

	err := newFast(3).Do(t.Context(), func() error {
		calls++
		return want
	})

	if !errors.Is(err, want) {
		t.Errorf("Do() error = %v, want %v", err, want)
	}
	if calls != 1 {
		t.Errorf("calls = %d, want 1", calls)
	}
}

func TestDoTemporaryFalseStopsImmediately(t *testing.T) {
	calls := 0
	err := newFast(3).Do(t.Context(), func() error {
		calls++
		// The message matches the substring list, but Temporary() is false.
		return &tempErr{msg: "connection timeout", temp: false}
	})

	if err == nil {
		t.Fatal("Do() returned nil, want an error")
	}
	if calls != 1 {
		t.Errorf("calls = %d, want 1", calls)
	}
}

func TestCalculateBackoffGrows(t *testing.T) {
	r := New(5, 3600)

	var previous time.Duration
	for attempt := 0; attempt < 5; attempt++ {
		got := r.calculateBackoff(attempt)

		// Attempt n waits 2^(n+1) seconds plus up to one second of jitter.
		floor := time.Duration(1<<(attempt+1)) * time.Second
		ceiling := floor + time.Second

		if got < floor || got > ceiling {
			t.Errorf("calculateBackoff(%d) = %v, want between %v and %v", attempt, got, floor, ceiling)
		}
		if got <= previous {
			t.Errorf("calculateBackoff(%d) = %v, want more than the previous %v", attempt, got, previous)
		}
		previous = got
	}
}

func TestCalculateBackoffCaps(t *testing.T) {
	r := New(10, 5)

	for attempt := 0; attempt < 10; attempt++ {
		got := r.calculateBackoff(attempt)
		if got > 5*time.Second {
			t.Errorf("calculateBackoff(%d) = %v, want 5s or less", attempt, got)
		}
	}
}

func TestCalculateBackoffZeroMax(t *testing.T) {
	r := New(3, 0)

	for attempt := 0; attempt < 3; attempt++ {
		if got := r.calculateBackoff(attempt); got != 0 {
			t.Errorf("calculateBackoff(%d) = %v, want 0", attempt, got)
		}
	}
}

// TestDoRetriesOnTheTemporaryFlagAlone confirms the classifier reads the type,
// not the message. The earlier code also required the text to match a substring
// list, so an unrecognized message stopped the retry.
func TestDoRetriesOnTheTemporaryFlagAlone(t *testing.T) {
	calls := 0
	err := newFast(3).Do(t.Context(), func() error {
		calls++
		return &tempErr{msg: "boom", temp: true}
	})

	if err == nil {
		t.Fatal("Do() returned nil, want an error")
	}
	if calls != 4 {
		t.Errorf("calls = %d, want 4", calls)
	}
}

// TestDoRetriesAWrappedTemporaryError confirms the classifier looks through a
// wrapper, so a caller may add context to the error.
func TestDoRetriesAWrappedTemporaryError(t *testing.T) {
	calls := 0
	err := newFast(2).Do(t.Context(), func() error {
		calls++
		return fmt.Errorf("fetching page 2: %w", &tempErr{msg: "boom", temp: true})
	})

	if err == nil {
		t.Fatal("Do() returned nil, want an error")
	}
	if calls != 3 {
		t.Errorf("calls = %d, want 3", calls)
	}
}

func TestIsTemporary(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"nil", nil, false},
		{"plain error", errors.New("boom"), false},
		{"temporary", &tempErr{msg: "boom", temp: true}, true},
		{"not temporary", &tempErr{msg: "boom", temp: false}, false},
		{"wrapped temporary", fmt.Errorf("context: %w", &tempErr{msg: "boom", temp: true}), true},
		{"wrapped permanent", fmt.Errorf("context: %w", errors.New("boom")), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsTemporary(tt.err); got != tt.want {
				t.Errorf("IsTemporary(%v) = %v, want %v", tt.err, got, tt.want)
			}
		})
	}
}

// TestDoStopsWhenTheContextIsCancelled confirms a cancelled context ends the
// wait instead of sleeping through it.
func TestDoStopsWhenTheContextIsCancelled(t *testing.T) {
	// A 30 second cap makes the first wait far longer than the test allows.
	r := New(5, 30)

	ctx, cancel := context.WithCancel(t.Context())
	calls := 0

	start := time.Now()
	err := r.Do(ctx, func() error {
		calls++
		cancel() // Cancel while the retryer is about to wait.
		return &tempErr{msg: "boom", temp: true}
	})
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("Do() returned nil, want an error")
	}
	if elapsed > 500*time.Millisecond {
		t.Errorf("elapsed = %v, want under 500ms", elapsed)
	}
	if calls != 1 {
		t.Errorf("calls = %d, want 1", calls)
	}
}

// TestDoDoesNotCallFnWithACancelledContext confirms the loop checks the context
// before the first attempt.
func TestDoDoesNotCallFnWithACancelledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	calls := 0
	err := newFast(3).Do(ctx, func() error {
		calls++
		return nil
	})

	if !errors.Is(err, context.Canceled) {
		t.Errorf("error = %v, want context.Canceled", err)
	}
	if calls != 0 {
		t.Errorf("calls = %d, want 0", calls)
	}
}
