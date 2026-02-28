FROM golang:1.25.5-alpine AS builder

RUN apk add --no-cache tzdata

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o clickhouse_gateway_api .

FROM gcr.io/distroless/base-debian12

WORKDIR /app

COPY --from=builder /app/clickhouse_gateway_api .

COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo
ENV TZ=America/Sao_Paulo

USER nonroot:nonroot

ENTRYPOINT ["/app/clickhouse_gateway_api"]
