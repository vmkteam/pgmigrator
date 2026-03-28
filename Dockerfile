FROM golang:1.25-alpine AS builder
COPY . /build
RUN cd /build && go install -mod=mod

FROM alpine:latest

COPY --from=builder /go/bin/pgmigrator .
