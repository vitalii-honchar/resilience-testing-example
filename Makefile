
.PHONY: test

test:
	go test -v ./...

gofumpt:
	go gofumpt -w .