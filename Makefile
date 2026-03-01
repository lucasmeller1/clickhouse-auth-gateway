COMPOSE = docker compose -f ./deploy/docker-compose.yml

.PHONY: build up down down-clean restart deploy clickhouse

clickhouse:
	docker build --rm -f ./deploy/clickhouse/clickhouse_dockerfile -t clickhousedb ./deploy/clickhouse/

build:
		docker build -t clickhouse_auth_gateway:latest .

up:
		$(COMPOSE) up -d

down:
		$(COMPOSE) down

down-clean:
		$(COMPOSE) down -v

restart: down up

deploy: build clickhouse restart
