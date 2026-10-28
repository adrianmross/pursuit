.PHONY: test build run web

web:
	cd web && npm install && npm run build
	rm -rf internal/dashboard/dist
	cp -r web/dist internal/dashboard/dist

test: web
	go test ./...

build: web
	CGO_ENABLED=0 go build -o bin/pursuit ./cmd/pursuit

run: build
	./bin/pursuit serve
