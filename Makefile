build:
		docker build -t excel_api:latest .

up:
		docker compose -f docker-compose.yml up -d

down:
		docker compose -f docker-compose.yml down -v

auto: down up

all: build auto
