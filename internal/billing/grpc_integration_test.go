package billing

import (
	"testing"

	billingv1 "github.com/rexemtoxa/gepard_billing/proto/billing/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestApplyOperationInsufficientFundsIntegration(t *testing.T) {
	session := billingIntegrationSessionForTest(t)

	response, err := session.applyOperation(&billingv1.ApplyOperationRequest{
		Source: "payment",
		State:  "withdraw",
		Amount: "99.99",
		TxId:   "insufficient-funds-1",
	})
	if err != nil {
		t.Fatalf("ApplyOperation returned error: %v", err)
	}
	if response.GetResultStatus() != apiResultStatusRejectedInsufficientFunds {
		t.Fatalf("result_status = %q, want %q", response.GetResultStatus(), apiResultStatusRejectedInsufficientFunds)
	}
	if response.GetDuplicate() {
		t.Fatalf("duplicate = %v, want false", response.GetDuplicate())
	}

	state := session.dbState()
	if state.OperationRequests != 1 {
		t.Fatalf("operation_requests = %d, want %d", state.OperationRequests, 1)
	}
	if state.LedgerEntries != 0 {
		t.Fatalf("ledger_entries = %d, want %d", state.LedgerEntries, 0)
	}
	if state.Balance != "0" {
		t.Fatalf("balance = %q, want %q", state.Balance, "0")
	}
}

func TestApplyOperationNegativeAmountIntegration(t *testing.T) {
	session := billingIntegrationSessionForTest(t)

	_, err := session.applyOperation(&billingv1.ApplyOperationRequest{
		Source: "game",
		State:  "deposit",
		Amount: "-10",
		TxId:   "negative-amount-1",
	})
	if err == nil {
		t.Fatal("expected validation error, got nil")
	}
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("status code = %s, want %s", status.Code(err), codes.InvalidArgument)
	}

	state := session.dbState()
	if state.OperationRequests != 0 {
		t.Fatalf("operation_requests = %d, want %d", state.OperationRequests, 0)
	}
	if state.LedgerEntries != 0 {
		t.Fatalf("ledger_entries = %d, want %d", state.LedgerEntries, 0)
	}
	if state.Balance != "0" {
		t.Fatalf("balance = %q, want %q", state.Balance, "0")
	}
}

func TestApplyOperationUnknownSourceIntegration(t *testing.T) {
	session := billingIntegrationSessionForTest(t)

	_, err := session.applyOperation(&billingv1.ApplyOperationRequest{
		Source: "client",
		State:  "deposit",
		Amount: "10",
		TxId:   "unknown-source-1",
	})
	if err == nil {
		t.Fatal("expected validation error, got nil")
	}
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("status code = %s, want %s", status.Code(err), codes.InvalidArgument)
	}

	state := session.dbState()
	if state.OperationRequests != 0 {
		t.Fatalf("operation_requests = %d, want %d", state.OperationRequests, 0)
	}
	if state.LedgerEntries != 0 {
		t.Fatalf("ledger_entries = %d, want %d", state.LedgerEntries, 0)
	}
	if state.Balance != "0" {
		t.Fatalf("balance = %q, want %q", state.Balance, "0")
	}
}

func TestApplyOperationIdempotencyIntegration(t *testing.T) {
	session := billingIntegrationSessionForTest(t)

	firstResponse, err := session.applyOperation(&billingv1.ApplyOperationRequest{
		Source: "game",
		State:  "deposit",
		Amount: "10",
		TxId:   "dup-1",
	})
	if err != nil {
		t.Fatalf("first ApplyOperation returned error: %v", err)
	}
	if firstResponse.GetResultStatus() != apiResultStatusApplied {
		t.Fatalf("first result_status = %q, want %q", firstResponse.GetResultStatus(), apiResultStatusApplied)
	}
	if firstResponse.GetDuplicate() {
		t.Fatalf("first duplicate = %v, want false", firstResponse.GetDuplicate())
	}

	secondResponse, err := session.applyOperation(&billingv1.ApplyOperationRequest{
		Source: "game",
		State:  "deposit",
		Amount: "999.99",
		TxId:   "dup-1",
	})
	if err != nil {
		t.Fatalf("second ApplyOperation returned error: %v", err)
	}
	if secondResponse.GetResultStatus() != apiResultStatusApplied {
		t.Fatalf("second result_status = %q, want %q", secondResponse.GetResultStatus(), apiResultStatusApplied)
	}
	if !secondResponse.GetDuplicate() {
		t.Fatalf("second duplicate = %v, want true", secondResponse.GetDuplicate())
	}

	state := session.dbState()
	if state.OperationRequests != 1 {
		t.Fatalf("operation_requests = %d, want %d", state.OperationRequests, 1)
	}
	if state.LedgerEntries != 1 {
		t.Fatalf("ledger_entries = %d, want %d", state.LedgerEntries, 1)
	}
	if state.Balance != "10" {
		t.Fatalf("balance = %q, want %q", state.Balance, "10")
	}
}
