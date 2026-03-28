PKG := `go list -f {{.Dir}} ./...`

LINT_VERSION := v2.8.0

fmt:
	@golangci-lint fmt

lint:
	@golangci-lint version
	@golangci-lint config verify
	@golangci-lint run

test:
	@go test -v ./...

db-test:
	@dropdb --if-exists pgmigrator
	@createdb pgmigrator

mod:
	@go mod tidy

build:
	@CGO_ENABLED=0 go build -o pgmigrator
