FROM golang:1.22-alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY main.go ./
RUN CGO_ENABLED=0 GOOS=linux go build -o /service main.go

FROM alpine:3.20

RUN apk add --no-cache ca-certificates
COPY --from=builder /service /service

EXPOSE 8083

ENTRYPOINT ["/service"]
