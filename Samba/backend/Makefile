.PHONY: build vet test run

build:
	go build ./...

vet:
	go vet ./...

test:
	go test ./...

run:
	set -a; [ -f .env ] && . ./.env; set +a; go run ./cmd/samba-admin
