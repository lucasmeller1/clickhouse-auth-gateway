build:
		CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o ./prod/excel_api

up:
		docker compose -f ./prod/docker-compose.yml up -d --build

down:
		docker compose -f ./prod/docker-compose.yml down

auto: down up

all: build auto
