.PHONY: run migrate test integration vet build
run:
	go run ./cmd/order-service
migrate:
	go run ./cmd/order-service migrate
test:
	go test -count=1 ./...
integration:
	@test -n "$$TEST_DATABASE_URL" || (echo 'Set TEST_DATABASE_URL to a dedicated test database'; exit 1)
	go test -count=1 -v ./tests
vet:
	go vet ./...
build:
	go build -o bin/order-service ./cmd/order-service
