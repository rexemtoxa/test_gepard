package billingworker

import (
	"context"
	"errors"
	"testing"
	"time"
)

type recordingCanceler struct {
	count int
	err   error

	lastLimit    int32
	deadlineSet  bool
	timeoutValue time.Duration
}

func (r *recordingCanceler) CancelLatestOddOperations(ctx context.Context, limit int32) (int, error) {
	r.lastLimit = limit

	deadline, ok := ctx.Deadline()
	r.deadlineSet = ok
	if ok {
		r.timeoutValue = time.Until(deadline)
	}

	return r.count, r.err
}

func TestNewSchedulerRejectsInvalidSpec(t *testing.T) {
	t.Parallel()

	_, err := NewScheduler("not-a-cron-spec", &recordingCanceler{})
	if err == nil {
		t.Fatal("NewScheduler returned nil error")
	}
}

func TestRunCancelLatestOddOperationsUsesConfiguredLimitAndTimeout(t *testing.T) {
	t.Parallel()

	canceler := &recordingCanceler{count: 1}

	runCancelLatestOddOperations(canceler, 2*time.Second, 25)

	if canceler.lastLimit != 25 {
		t.Fatalf("lastLimit = %d, want %d", canceler.lastLimit, 25)
	}
	if !canceler.deadlineSet {
		t.Fatal("expected deadline to be set")
	}
	if canceler.timeoutValue <= 0 {
		t.Fatalf("timeoutValue = %s, want > 0", canceler.timeoutValue)
	}
	if canceler.timeoutValue > 2*time.Second {
		t.Fatalf("timeoutValue = %s, want <= %s", canceler.timeoutValue, 2*time.Second)
	}
}

func TestRunCancelLatestOddOperationsSwallowsServiceError(t *testing.T) {
	t.Parallel()

	canceler := &recordingCanceler{err: errors.New("boom")}

	runCancelLatestOddOperations(canceler, time.Second, 10)

	if canceler.lastLimit != 10 {
		t.Fatalf("lastLimit = %d, want %d", canceler.lastLimit, 10)
	}
}
