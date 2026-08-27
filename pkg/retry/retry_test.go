package retry

import (
	"errors"
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
	err := newFast(3).Do(func() error {
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
	err := newFast(3).Do(func() error {
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
			err := newFast(tt.maxRetries).Do(func() error {
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

	err := newFast(3).Do(func() error {
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

// TestDoTemporaryFlagAloneDoesNotRetry records a defect. Do checks the
// Temporary() method, then also requires the error text to match a substring
// list. An error that reports Temporary() true but carries an unrecognized
// message is not retried.
//
// Delete this test when Do classifies errors by type alone.
func TestDoTemporaryFlagAloneDoesNotRetry(t *testing.T) {
	calls := 0
	err := newFast(3).Do(func() error {
		calls++
		return &tempErr{msg: "boom", temp: true}
	})

	if err == nil {
		t.Fatal("Do() returned nil, want an error")
	}
	if calls != 1 {
		t.Errorf("calls = %d, want 1; Do now honors Temporary() alone, so update this test", calls)
	}
}

func TestDoTemporaryFalseStopsImmediately(t *testing.T) {
	calls := 0
	err := newFast(3).Do(func() error {
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

func TestIsLikelyTemporary(t *testing.T) {
	tests := []struct {
		msg  string
		want bool
	}{
		{"temporary error with status 503", true},
		{"API request failed with status 429", true},
		{"too many requests", true},
		{"gateway timeout", true},
		{"connection refused", true},
		{"connection reset by peer", true},
		{"service unavailable", true},
		{"request failed: dial tcp: i/o timeout", true},
		{"API request failed with status 404", false},
		{"API request failed with status 401", false},
		{"failed to marshal request body", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.msg, func(t *testing.T) {
			if got := isLikelyTemporary(tt.msg); got != tt.want {
				t.Errorf("isLikelyTemporary(%q) = %v, want %v", tt.msg, got, tt.want)
			}
		})
	}
}

// TestIsLikelyTemporaryMatchesUnrelatedText records that the check is a
// substring scan over the message, so unrelated wording can trigger a retry.
func TestIsLikelyTemporaryMatchesUnrelatedText(t *testing.T) {
	if !isLikelyTemporary("failed to decode response: unexpected timeout field") {
		t.Error("expected the substring scan to match; the classifier changed, so update this test")
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

// TestDoIgnoresCancellation records that Do has no context parameter, so a
// caller cannot stop the backoff sleep. The test measures that the sleep runs.
//
// Delete this test when Do accepts a context.
func TestDoIgnoresCancellation(t *testing.T) {
	r := New(1, 1)

	start := time.Now()
	_ = r.Do(func() error {
		return &tempErr{msg: "temporary error with status 500", temp: true}
	})
	elapsed := time.Since(start)

	if elapsed < 900*time.Millisecond {
		t.Errorf("elapsed = %v, want at least 900ms; Do no longer sleeps, so update this test", elapsed)
	}
}
