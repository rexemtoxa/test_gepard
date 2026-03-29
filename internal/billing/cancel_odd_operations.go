package billing

import (
	"context"
	"database/sql"

	"github.com/rexemtoxa/gepard_billing/internal/repository"
)

func (s *Service) CancelLatestOddOperations(ctx context.Context, limit int32) (int, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
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
		return 0, err
	}
	if !workerLock {
		if err := tx.Commit(); err != nil {
			return 0, err
		}
		committed = true
		return 0, nil
	}

	if err := queries.LockBalanceMutations(ctx, repository.LockBalanceMutationsParams{
		LockGroup: lockGroupBalance,
		LockKey:   lockKeyBalance,
	}); err != nil {
		return 0, err
	}

	head, err := queries.GetLedgerHead(ctx)
	if err != nil {
		return 0, err
	}

	currentBalance, err := ParseMoney(head.BalanceAfterText)
	if err != nil {
		return 0, err
	}
	currentHeadID := nullableInt64(head.ID)

	candidates, err := queries.ListCancelCandidates(ctx, limit)
	if err != nil {
		return 0, err
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
			return 0, err
		}

		currentBalance = nextBalance
		currentHeadID = sql.NullInt64{Int64: insertedEntry.ID, Valid: true}
		canceledCount++
	}

	if err := tx.Commit(); err != nil {
		return 0, err
	}
	committed = true

	return canceledCount, nil
}
