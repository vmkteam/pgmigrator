FROM golang:1.20-alpine AS builder
COPY . /build
RUN cd /build && go install -mod=mod

FROM alpine:latest

COPY --from=builder /go/bin/pgmigrator .
