.PHONY: test build run web

web:
	cd web && npm install && npm run build
	rm -rf internal/dashboard/dist
	cp -r web/dist internal/dashboard/dist
	cd docs-site && npm install && npm run build
	cp -r docs-site/out internal/dashboard/dist/docs

test: web
	go test ./...

build: web
	CGO_ENABLED=0 go build -o bin/pursuit ./cmd/pursuit

run: build
	./bin/pursuit serve
