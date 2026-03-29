ARG GO_VERSION=1.26.1
ARG BUF_VERSION=1.66.1
ARG SQLC_VERSION=1.30.0

FROM bufbuild/buf:${BUF_VERSION} AS proto-gen
WORKDIR /workspace

COPY buf.yaml buf.gen.yaml ./
COPY proto ./proto

RUN ["buf", "generate"]

FROM sqlc/sqlc:${SQLC_VERSION} AS sqlc-gen
WORKDIR /src

COPY sqlc.yaml ./
COPY db ./db

RUN ["/workspace/sqlc", "generate"]

FROM golang:${GO_VERSION}-bookworm AS builder
WORKDIR /app

COPY go.mod go.sum ./

RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

COPY cmd ./cmd
COPY internal ./internal
COPY --from=proto-gen /workspace/proto ./proto
COPY --from=sqlc-gen /src/internal/repository ./internal/repository

RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 \
    go build -trimpath -ldflags="-s -w" -o /out/gepard-billing ./cmd/server

FROM gcr.io/distroless/static-debian12:nonroot AS runtime
WORKDIR /app

ENV GRPC_PORT=50051
ENV SERVICE_NAME=gepard-billing

COPY --from=builder /out/gepard-billing /app/gepard-billing

EXPOSE 50051

ENTRYPOINT ["/app/gepard-billing"]
