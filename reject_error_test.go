package rabbitmq

import (
	"errors"
	"testing"
)

// A RejectError without a Cause is easy to construct and is logged by the
// consumer on every reject; Error must never panic on it.
func TestRejectErrorWithoutCauseDoesNotPanic(t *testing.T) {
	for _, requeue := range []bool{true, false} {
		r := &RejectError{Requeue: requeue}
		var msg string
		func() {
			defer func() {
				if p := recover(); p != nil {
					t.Fatalf("RejectError{Requeue:%v}.Error() panicked: %v", requeue, p)
				}
			}()
			msg = r.Error()
		}()
		if msg == "" {
			t.Fatalf("RejectError{Requeue:%v}.Error() returned an empty string", requeue)
		}
		if r.Unwrap() != nil {
			t.Fatalf("Unwrap of a cause-less RejectError = %v, want nil", r.Unwrap())
		}
	}
}

func TestRejectErrorWithCauseKeepsItsMessage(t *testing.T) {
	cause := errors.New("boom")
	r := &RejectError{Requeue: true, Cause: cause}
	if r.Error() != "boom" {
		t.Fatalf("Error() = %q, want %q", r.Error(), "boom")
	}
	if !errors.Is(r, cause) {
		t.Fatal("errors.Is(r, cause) = false, want true")
	}
}
