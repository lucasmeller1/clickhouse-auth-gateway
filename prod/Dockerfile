# docker build -t excel_api:latest .
# CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o excel_api
FROM gcr.io/distroless/base-debian12

WORKDIR /app

COPY excel_api /app/excel_api

USER nonroot:nonroot

ENTRYPOINT ["/app/excel_api"]

