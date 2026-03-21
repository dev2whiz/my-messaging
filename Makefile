.PHONY: help up down restart ps logs infra-up infra-down infra-clean migrate gateway-run gateway-test gateway-build web-install web-run web-build build test smoke smoke-local check clean

help:
	@printf "Available targets:\n"
	@printf "  make up            # full docker compose stack\n"
	@printf "  make down          # stop docker compose stack\n"
	@printf "  make restart       # rebuild and restart docker compose stack\n"
	@printf "  make ps            # docker compose service status\n"
	@printf "  make logs          # tail compose logs\n"
	@printf "  make infra-up      # Docker: start postgres + valkey + rabbitmq only\n"
	@printf "  make infra-down    # Docker: stop postgres + valkey + rabbitmq only\n"
	@printf "  make infra-clean   # Docker: destroy compose infra, volumes, caches, and networks\n"
	@printf "  make migrate       # Docker: run DB migrations container\n"
	@printf "  make gateway-run   # Local process: run Spring Boot locally\n"
	@printf "  make gateway-test  # Local process: run Spring tests\n"
	@printf "  make gateway-build # Local process: build Spring artifact\n"
	@printf "  make web-install   # Local process: install frontend deps\n"
	@printf "  make web-run       # Local process: run Vite dev server\n"
	@printf "  make web-build     # Local process: build frontend\n"
	@printf "  make build         # Local process: build backend + frontend\n"
	@printf "  make test          # Local process: backend tests + frontend build check\n"
	@printf "  make smoke         # Docker-backed integration smoke test\n"
	@printf "  make smoke-local   # Local app processes + Docker infra smoke test\n"
	@printf "  make check         # Local process tests + Docker smoke test\n"
	@printf "  make clean         # Local process: clean backend + frontend build outputs\n"

up:
	docker compose up --build -d

down:
	docker compose down

restart:
	docker compose up --build -d

ps:
	docker compose ps

logs:
	docker compose logs --tail=200 -f

infra-up:
	docker compose up -d postgres valkey rabbitmq

infra-down:
	docker compose stop postgres valkey rabbitmq

infra-clean:
	docker compose down -v --remove-orphans

migrate:
	docker compose up migrate

gateway-run:
	$(MAKE) -C services/gateway-spring run

gateway-test:
	$(MAKE) -C services/gateway-spring test

gateway-build:
	$(MAKE) -C services/gateway-spring build

web-install:
	$(MAKE) -C web install

web-run:
	$(MAKE) -C web run

web-build:
	$(MAKE) -C web build

build: gateway-build web-build

test: gateway-test web-build

smoke:
	./scripts/smoke-test.sh docker

smoke-local:
	./scripts/smoke-test.sh local

check: test smoke

clean:
	$(MAKE) -C services/gateway-spring clean
	$(MAKE) -C web clean