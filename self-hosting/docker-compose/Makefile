.PHONY: up down logs ps restart config pull build up-build recreate rebuild platform-up opentaco-up all-up

PROFILE ?= all
PROJECT_ROOT ?= $(abspath ../..)
COMPOSE_FILE := $(abspath docker-compose.yml)
DOCKER_COMPOSE = docker compose --project-directory $(PROJECT_ROOT) -f $(COMPOSE_FILE)

up:
	$(DOCKER_COMPOSE) --profile $(PROFILE) up -d

down:
	$(DOCKER_COMPOSE) --profile $(PROFILE) down

logs:
	$(DOCKER_COMPOSE) --profile $(PROFILE) logs -f

ps:
	$(DOCKER_COMPOSE) ps

restart:
	$(DOCKER_COMPOSE) --profile $(PROFILE) restart

config:
	$(DOCKER_COMPOSE) config

pull:
	$(DOCKER_COMPOSE) --profile $(PROFILE) pull

build:
	$(DOCKER_COMPOSE) --profile $(PROFILE) build

up-build:
	$(DOCKER_COMPOSE) --profile $(PROFILE) up -d --build

recreate:
	$(DOCKER_COMPOSE) --profile $(PROFILE) up -d --force-recreate

rebuild:
	$(DOCKER_COMPOSE) --profile $(PROFILE) up -d --build --force-recreate

platform-up: PROFILE=platform
platform-up: up

opentaco-up: PROFILE=opentaco
opentaco-up: up

all-up: PROFILE=all
all-up: up
