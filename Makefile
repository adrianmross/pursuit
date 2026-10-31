.PHONY: test build linux run web openapi

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

# Regenerate api/openapi.json from internal/spec; go test fails when it drifts.
openapi:
	UPDATE_OPENAPI=1 go test -count=1 ./internal/spec -run TestOpenAPIFile
