package billing

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/rexemtoxa/gepard_billing/internal/repository"
)

func (s *Service) CancelLatestOddOperations(ctx context.Context, limit int32) (int, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("begin cancellation transaction: %w", err)
	}

	queries := s.queries.WithTx(tx)
	committed := false

	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()

	workerLock, err := queries.TryLockCancellationJob(ctx, repository.TryLockCancellationJobParams{
		LockGroup: lockGroupWorker,
		LockKey:   lockKeyWorker,
	})
	if err != nil {
		return 0, fmt.Errorf("acquire cancellation worker lock: %w", err)
	}

	if !workerLock {
		err = tx.Commit()
		if err != nil {
			return 0, fmt.Errorf("commit unlocked cancellation transaction: %w", err)
		}

		committed = true

		return 0, nil
	}

	err = queries.LockBalanceMutations(ctx, repository.LockBalanceMutationsParams{
		LockGroup: lockGroupBalance,
		LockKey:   lockKeyBalance,
	})
	if err != nil {
		return 0, fmt.Errorf("lock balance mutations: %w", err)
	}

	head, err := queries.GetLedgerHead(ctx)
	if err != nil {
		return 0, fmt.Errorf("get ledger head: %w", err)
	}

	currentBalance, err := ParseMoney(head.BalanceAfterText)
	if err != nil {
		return 0, err
	}

	currentHeadID := nullableInt64(head.ID)

	candidates, err := queries.ListCancelCandidates(ctx, limit)
	if err != nil {
		return 0, fmt.Errorf("list cancellation candidates: %w", err)
	}

	canceledCount := 0

	for _, candidate := range candidates {
		signedAmount, err := ParseMoney(candidate.LeSignedAmount)
		if err != nil {
			return 0, err
		}

		reversal := signedAmount.Neg()
		nextBalance := currentBalance.Add(reversal)

		if nextBalance.IsNegative() {
			continue
		}

		insertedEntry, err := queries.InsertCancelLedgerEntry(ctx, repository.InsertCancelLedgerEntryParams{
			ReversesEntryID: sql.NullInt64{Int64: candidate.ID, Valid: true},
			SignedAmount:    reversal.String(),
			PrevEntryID:     currentHeadID,
			BalanceAfter:    nextBalance.String(),
		})
		if err != nil {
			return 0, fmt.Errorf("insert cancel ledger entry: %w", err)
		}

		currentBalance = nextBalance
		currentHeadID = sql.NullInt64{Int64: insertedEntry.ID, Valid: true}
		canceledCount++
	}

	err = tx.Commit()
	if err != nil {
		return 0, fmt.Errorf("commit cancellation transaction: %w", err)
	}

	committed = true

	return canceledCount, nil
}
