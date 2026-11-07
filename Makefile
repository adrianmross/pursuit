.PHONY: test build linux run web openapi lint-openapi

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

# Validate api/openapi.json with the Redocly CLI pinned in scripts/openapi-lint (redocly.yaml).
lint-openapi:
	npm ci --prefix scripts/openapi-lint --ignore-scripts --no-audit --no-fund
	scripts/openapi-lint/node_modules/.bin/redocly lint api/openapi.json
