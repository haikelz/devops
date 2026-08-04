FROM golang:1.23-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_NEABLED=0 go build -o ekel-backend .

FROM alpine:3.21
RUN apk add --no-cache ca-certificates tzdata
COPY --from=builder /app/ekel-backend /usr/bin/ekel-backend
USER 65532
EXPOSE 1323
ENTRYPOINT ["/usr/bin/ekel-backend"]