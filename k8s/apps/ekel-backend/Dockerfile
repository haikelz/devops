FROM golang:1.25-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY cmd ./cmd
COPY docs ./docs
COPY internal ./internal

RUN CGO_ENABLED=0 go build -ldflags="-w -s" -o /app/main ./cmd/app/development

FROM alpine:latest

RUN apk --no-cache add ca-certificates

WORKDIR /app

COPY --from=builder /app/main ./main

EXPOSE 9090

CMD ["./main"]
