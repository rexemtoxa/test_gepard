package billing

import (
	"database/sql"

	"github.com/rexemtoxa/gepard_billing/internal/repository"
)

const (
	lockGroupBalance int32 = 1
	lockKeyBalance   int32 = 1
	lockGroupWorker  int32 = 1
	lockKeyWorker    int32 = 2

	dbResultStatusApplied                   = "applied"
	dbResultStatusRejectedInsufficientFunds = "rejected_insufficient_funds"

	apiResultStatusApplied                   = "APPLIED"
	apiResultStatusRejectedInsufficientFunds = "REJECTED_INSUFFICIENT_FUNDS"

	sourceGame    = "game"
	sourcePayment = "payment"
	sourceService = "service"

	stateDeposit  = "deposit"
	stateWithdraw = "withdraw"
)

type Service struct {
	db      *sql.DB
	queries *repository.Queries
}

func NewService(db *sql.DB) *Service {
	return &Service{
		db:      db,
		queries: repository.New(db),
	}
}

func nullableInt64(id int64) sql.NullInt64 {
	if id == 0 {
		return sql.NullInt64{}
	}

	return sql.NullInt64{Int64: id, Valid: true}
}
