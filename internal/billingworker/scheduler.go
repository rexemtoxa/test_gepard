package billingworker

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/robfig/cron/v3"
)

const (
	cancelLatestOddOperationsLimit   int32         = 10
	cancelLatestOddOperationsTimeout time.Duration = 15 * time.Second
)

type oddOperationCanceler interface {
	CancelLatestOddOperations(ctx context.Context, limit int32) (int, error)
}

type Scheduler struct {
	scheduler *cron.Cron
}

func NewScheduler(spec string, canceler oddOperationCanceler) (*Scheduler, error) {
	scheduler := cron.New()

	_, err := scheduler.AddFunc(spec, func() {
		runCancelLatestOddOperations(canceler, cancelLatestOddOperationsTimeout, cancelLatestOddOperationsLimit)
	})
	if err != nil {
		return nil, fmt.Errorf("schedule cancellation worker: %w", err)
	}

	return &Scheduler{scheduler: scheduler}, nil
}

func (s *Scheduler) Start() {
	s.scheduler.Start()
}

func (s *Scheduler) Stop() {
	stopCtx := s.scheduler.Stop()
	<-stopCtx.Done()
}

func runCancelLatestOddOperations(canceler oddOperationCanceler, timeout time.Duration, limit int32) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	canceledCount, err := canceler.CancelLatestOddOperations(ctx, limit)
	if err != nil {
		log.Printf("cancel latest odd operations: %v", err)

		return
	}

	if canceledCount > 0 {
		log.Printf("canceled %d odd operations", canceledCount)
	}
}
