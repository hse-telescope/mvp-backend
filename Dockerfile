FROM golang:alpine AS builder
WORKDIR /app
COPY . .
RUN apk add --no-cache make
RUN go build -ldflags "-s -w" -o ./bin/mvp-backend ./cmd

FROM alpine:latest AS runner
WORKDIR /app
COPY --from=builder /app/bin/mvp-backend ./mvp-backend
COPY migrations migrations

ENTRYPOINT ["./mvp-backend"]
