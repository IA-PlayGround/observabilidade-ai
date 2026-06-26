FROM golang:1.22-alpine AS builder

WORKDIR /app
COPY go.mod ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -mod=mod -o /ai-engine ./cmd

FROM alpine:3.20

RUN apk add --no-cache ca-certificates tzdata
COPY --from=builder /ai-engine /ai-engine

EXPOSE 8082

ENTRYPOINT ["/ai-engine"]
