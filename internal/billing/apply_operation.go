package billing

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/rexemtoxa/gepard_billing/internal/repository"
)

type ApplyCommand struct {
	Source string
	State  string
	Amount Money
	TxID   string
}

type ApplyResult struct {
	TxID         string
	ResultStatus string
	Duplicate    bool
}

func ValidateApplyCommand(source, state, amount, txID string) (ApplyCommand, error) {
	if txID == "" {
		return ApplyCommand{}, fmt.Errorf("tx_id is required")
	}
	if source != sourceGame && source != sourcePayment && source != sourceService {
		return ApplyCommand{}, fmt.Errorf("source must be one of game, payment, service")
	}
	if state != stateDeposit && state != stateWithdraw {
		return ApplyCommand{}, fmt.Errorf("state must be one of deposit, withdraw")
	}

	parsedAmount, err := ParsePositiveMoney(amount)
	if err != nil {
		return ApplyCommand{}, err
	}

	return ApplyCommand{
		Source: source,
		State:  state,
		Amount: parsedAmount,
		TxID:   txID,
	}, nil
}

func (s *Service) ApplyOperation(ctx context.Context, command ApplyCommand) (ApplyResult, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return ApplyResult{}, err
	}

	queries := s.queries.WithTx(tx)
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()

	if err := queries.LockBalanceMutations(ctx, repository.LockBalanceMutationsParams{
		LockGroup: lockGroupBalance,
		LockKey:   lockKeyBalance,
	}); err != nil {
		return ApplyResult{}, err
	}

	existingStatus, err := queries.GetOperationRequestResultStatus(ctx, command.TxID)
	switch {
	case err == nil:
		if err := tx.Commit(); err != nil {
			return ApplyResult{}, err
		}
		committed = true
		return applyResultFromValues(command.TxID, existingStatus, true), nil
	case !errors.Is(err, sql.ErrNoRows):
		return ApplyResult{}, err
	}

	head, err := queries.GetLedgerHead(ctx)
	if err != nil {
		return ApplyResult{}, err
	}

	currentBalance, err := ParseMoney(head.BalanceAfterText)
	if err != nil {
		return ApplyResult{}, err
	}

	newBalance := currentBalance
	dbStatus := dbResultStatusApplied
	if command.State == stateDeposit {
		newBalance = currentBalance.Add(command.Amount)
	} else {
		newBalance = currentBalance.Sub(command.Amount)
		if newBalance.IsNegative() {
			dbStatus = dbResultStatusRejectedInsufficientFunds
			newBalance = currentBalance
		}
	}

	insertedRequest, err := queries.InsertOperationRequest(ctx, repository.InsertOperationRequestParams{
		TxID:         command.TxID,
		Source:       command.Source,
		State:        command.State,
		Amount:       command.Amount.String(),
		ResultStatus: dbStatus,
	})
	if err != nil {
		return ApplyResult{}, err
	}

	if dbStatus == dbResultStatusApplied {
		prevEntryID := nullableInt64(head.ID)
		signedAmount := command.Amount
		if command.State == stateWithdraw {
			signedAmount = signedAmount.Neg()
		}

		_, err := queries.InsertApplyLedgerEntry(ctx, repository.InsertApplyLedgerEntryParams{
			RequestTxID:  sql.NullString{String: command.TxID, Valid: true},
			SignedAmount: signedAmount.String(),
			PrevEntryID:  prevEntryID,
			BalanceAfter: newBalance.String(),
		})
		if err != nil {
			return ApplyResult{}, err
		}

		if err := tx.Commit(); err != nil {
			return ApplyResult{}, err
		}
		committed = true

		return applyResultFromValues(insertedRequest.TxID, insertedRequest.ResultStatus, false), nil
	}

	if err := tx.Commit(); err != nil {
		return ApplyResult{}, err
	}
	committed = true

	return applyResultFromValues(insertedRequest.TxID, insertedRequest.ResultStatus, false), nil
}

func applyResultFromValues(txID, dbStatus string, duplicate bool) ApplyResult {
	return ApplyResult{
		TxID:         txID,
		ResultStatus: resultStatusToAPI(dbStatus),
		Duplicate:    duplicate,
	}
}

func resultStatusToAPI(dbStatus string) string {
	switch dbStatus {
	case dbResultStatusApplied:
		return apiResultStatusApplied
	case dbResultStatusRejectedInsufficientFunds:
		return apiResultStatusRejectedInsufficientFunds
	default:
		return dbStatus
	}
}
