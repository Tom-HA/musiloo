# Stage 1: Build stage
FROM golang:1.23-alpine AS build

WORKDIR /app

COPY . .
RUN go mod download

RUN CGO_ENABLED=0 GOOS=linux go build -o musiloo ./cmd/musiloo

FROM alpine:latest

WORKDIR /app

COPY --from=build /app/musiloo .

RUN apk --no-cache add ca-certificates tzdata

ENTRYPOINT ["/app/musiloo"]