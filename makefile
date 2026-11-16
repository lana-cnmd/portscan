cover:
	@go test -count=1 -coverprofile=coverage.out ./...
	@go tool cover -html=coverage.out
	@rm coverage.out

test:
	go test -race -count=30 ./...
