FROM golang:alpine AS builder
ENV GOTOOLCHAIN=auto
WORKDIR /app
COPY go.mod go.sum* ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /bin/baramail-api ./cmd/api
RUN CGO_ENABLED=0 GOOS=linux go build -o /bin/baramail-cli ./cmd/cli

FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata
WORKDIR /app
COPY --from=builder /bin/baramail-api /app/baramail-api
COPY --from=builder /bin/baramail-cli /app/baramail-cli
COPY migrations /app/migrations
EXPOSE 8080
CMD ["/app/baramail-api"]

