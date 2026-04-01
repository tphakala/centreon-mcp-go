# Build stage
FROM golang:1.26-alpine AS builder

WORKDIR /app

RUN apk add --no-cache git

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -o centreon-mcp-go .

# Final stage
FROM alpine:latest

WORKDIR /app

RUN apk add --no-cache ca-certificates

RUN addgroup -S appgroup && adduser -S appuser -G appgroup

COPY --from=builder /app/centreon-mcp-go .

RUN chown -R appuser:appgroup /app

USER appuser

ENTRYPOINT ["./centreon-mcp-go"]
