.PHONY: test build linux run web

web:
	sh scripts/vercel-build.sh

test: web
	go test ./...

build: web
	CGO_ENABLED=0 go build -o bin/pursuit ./cmd/pursuit

# Linux binary for the devenv container (devenv container build serve).
linux: web
	GOOS=linux CGO_ENABLED=0 go build -o bin/linux/pursuit ./cmd/pursuit

run: build
	./bin/pursuit serve
