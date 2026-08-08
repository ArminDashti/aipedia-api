# syntax=docker/dockerfile:1

FROM golang:1.22-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY cmd/ ./cmd/
COPY internal/ ./internal/
COPY migrations/ ./migrations/
RUN go mod tidy && CGO_ENABLED=0 go build -o /out/server ./cmd/server

FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata
WORKDIR /app
COPY --from=build /out/server /app/server
COPY --from=build /src/migrations /app/migrations
ENV ADDR=:8091
ENV MIGRATIONS_DIR=/app/migrations
ENV DATABASE_URL=/data/aipedia.db
EXPOSE 8091
CMD ["/app/server"]
