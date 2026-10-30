.PHONY: test build run web

web:
	sh scripts/vercel-build.sh

test: web
	go test ./...

build: web
	CGO_ENABLED=0 go build -o bin/pursuit ./cmd/pursuit

run: build
	./bin/pursuit serve
