.PHONY: build vet test test-integration run

build:
	go build ./...

vet:
	go vet ./...
	go vet -tags integration ./...

test:
	go test ./...

test-integration:
	go test -tags integration -count=1 ./...

run:
	set -a; [ -f .env ] && . ./.env; set +a; go run ./cmd/ldap-admin
