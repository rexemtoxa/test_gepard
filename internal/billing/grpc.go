package billing

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	billingv1 "github.com/rexemtoxa/gepard_billing/proto/billing/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type operationApplier interface {
	ApplyOperation(ctx context.Context, command ApplyCommand) (ApplyResult, error)
}

type GRPCServer struct {
	billingv1.UnimplementedBillingServiceServer

	applier operationApplier
}

func NewGRPCServer(applier operationApplier) *GRPCServer {
	return &GRPCServer{applier: applier}
}

func (s *GRPCServer) ApplyOperation(
	ctx context.Context,
	request *billingv1.ApplyOperationRequest,
) (*billingv1.ApplyOperationResponse, error) {
	command, err := ValidateApplyCommand(request.GetSource(), request.GetState(), request.GetAmount(), request.GetTxId())
	if err != nil {
		return nil, grpcStatusError(codes.InvalidArgument, err.Error())
	}

	result, err := s.applier.ApplyOperation(ctx, command)
	if err != nil {
		return nil, mapServiceError(err)
	}

	return &billingv1.ApplyOperationResponse{
		ResultStatus: result.ResultStatus,
		Duplicate:    result.Duplicate,
	}, nil
}

func mapServiceError(err error) error {
	switch {
	case errors.Is(err, context.Canceled):
		return grpcStatusError(codes.Canceled, context.Canceled.Error())
	case errors.Is(err, context.DeadlineExceeded):
		return grpcStatusError(codes.DeadlineExceeded, context.DeadlineExceeded.Error())
	case errors.Is(err, sql.ErrConnDone):
		return grpcStatusError(codes.Unavailable, "database unavailable")
	default:
		return grpcStatusError(codes.Internal, "internal server error")
	}
}

func grpcStatusError(code codes.Code, message string) error {
	return fmt.Errorf("%w", status.Error(code, message))
}
