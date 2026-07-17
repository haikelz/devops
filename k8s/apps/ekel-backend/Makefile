.PHONY: dev build test lint swagger docker-build docker-run

dev:
	air

build:
	go build -o bin/app ./cmd/app/development

test:
	go test ./... -v -cover

lint:
	golangci-lint run

swagger:
	swag init --parseDependency --parseInternal --generalInfo cmd/app/development/main.go --output docs

docker-build:
	docker build -t ekel-backend .

docker-run:
	docker run --rm -p 9090:9090 --env-file .env ekel-backend
