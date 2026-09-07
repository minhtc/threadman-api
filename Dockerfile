# syntax=docker/dockerfile:1

FROM golang:1.27-alpine AS builder

WORKDIR /src
RUN apk add --no-cache ca-certificates tzdata

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-w -s" -o /out/threadman-api ./cmd/server

FROM alpine:3.22

RUN apk add --no-cache ca-certificates tzdata \
    && adduser -D -u 1000 appuser

WORKDIR /app
COPY --from=builder /out/threadman-api /app/threadman-api
COPY database/schema.sql /app/database/schema.sql
COPY public /app/public

USER appuser
ENV PORT=8080
EXPOSE 8080

CMD ["/app/threadman-api"]
