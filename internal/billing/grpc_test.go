package billing

import (
	"context"
	"testing"

	billingv1 "github.com/rexemtoxa/gepard_billing/proto/billing/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type recordingApplier struct {
	called bool
	result ApplyResult
	err    error
}

func (r *recordingApplier) ApplyOperation(context.Context, ApplyCommand) (ApplyResult, error) {
	r.called = true
	return r.result, r.err
}

func TestGRPCServerApplyOperationValidation(t *testing.T) {
	t.Parallel()

	applier := &recordingApplier{}
	server := NewGRPCServer(applier)

	_, err := server.ApplyOperation(context.Background(), &billingv1.ApplyOperationRequest{
		Source: "client",
		State:  "deposit",
		Amount: "10.15",
		TxId:   "tx-1",
	})
	if err == nil {
		t.Fatalf("expected validation error, got nil")
	}
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("unexpected status code: %s", status.Code(err))
	}
	if applier.called {
		t.Fatalf("applier should not be called for invalid requests")
	}
}

func TestGRPCServerApplyOperationSuccess(t *testing.T) {
	t.Parallel()

	applier := &recordingApplier{
		result: ApplyResult{
			TxID:         "tx-1",
			ResultStatus: "APPLIED",
			Duplicate:    true,
		},
	}
	server := NewGRPCServer(applier)

	response, err := server.ApplyOperation(context.Background(), &billingv1.ApplyOperationRequest{
		Source: "game",
		State:  "deposit",
		Amount: "10.15",
		TxId:   "tx-1",
	})
	if err != nil {
		t.Fatalf("ApplyOperation returned error: %v", err)
	}
	if !applier.called {
		t.Fatalf("applier was not called")
	}
	if response.GetResultStatus() != "APPLIED" || !response.GetDuplicate() {
		t.Fatalf("unexpected response: %+v", response)
	}
}
