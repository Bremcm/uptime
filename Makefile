COMPOSE := docker compose --env-file .env -f deployments/docker-compose.yml

.PHONY: up down logs ps build

up:
	$(COMPOSE) up -d

down:
	$(COMPOSE) down

build:
	$(COMPOSE) up -d --build

logs:
	$(COMPOSE) logs -f

ps:
	$(COMPOSE) ps