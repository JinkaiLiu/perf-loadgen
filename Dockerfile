# Multi-stage build for vibeready binaries.
FROM golang:1.25-alpine AS builder
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /out/loadgen ./cmd/loadgen
RUN CGO_ENABLED=0 go build -o /out/loadgen-master ./cmd/loadgen-master
RUN CGO_ENABLED=0 go build -o /out/loadgen-worker ./cmd/loadgen-worker
RUN CGO_ENABLED=0 go build -o /out/mockserver ./cmd/mockserver

FROM alpine:3.21
RUN apk add --no-cache ca-certificates
COPY --from=builder /out/loadgen /usr/local/bin/loadgen
COPY --from=builder /out/loadgen-master /usr/local/bin/loadgen-master
COPY --from=builder /out/loadgen-worker /usr/local/bin/loadgen-worker
COPY --from=builder /out/mockserver /usr/local/bin/mockserver
ENTRYPOINT ["/usr/local/bin/loadgen"]
