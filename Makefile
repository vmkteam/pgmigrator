PKG := `go list -f {{.Dir}} ./...`

fmt:
	@goimports -local "github.com/vmkteam/pgmigrator" -l -w $(PKG)

lint:
	@golangci-lint run -c .golangci.yml

test:
	@go test -v ./...

db-test:
	@dropdb --if-exists pgmigrator
	@createdb pgmigrator

mod:
	@go mod tidy

build:
	@CGO_ENABLED=0 go build -o pgmigrator
