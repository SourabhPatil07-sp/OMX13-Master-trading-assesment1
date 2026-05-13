.PHONY: help up down build logs seed test-engine test-engine-race test-api test-frontend test-all psql restart-api restart-engine restart-frontend clean install-frontend install-api

# Default target
help:
	@echo "OmniMarket Control Tower (Makefile)"
	@echo "==================================="
	@echo "Available commands:"
	@echo "  make up               - Start all services (detached)"
	@echo "  make down             - Stop all services"
	@echo "  make build            - Rebuild and start all services"
	@echo "  make logs             - Tail logs for all services"
	@echo "  make seed             - Run the database seeder (populates categories & markets)"
	@echo "  make psql             - Open a PostgreSQL shell in the database container"
	@echo "  make test-engine      - Run the Go trading engine test suite"
	@echo "  make test-engine-race - Run the Go trading engine test suite with race detector (slower)"
	@echo "  make benchmark-engine - Run the Go trading engine benchmark suite"
	@echo "  make test-api         - Run the FastAPI backend test suite via Docker"
	@echo "  make test-frontend    - Run the React Vite test suite locally"
	@echo "  make test-all         - Run all tests (Engine, API, Frontend)"
	@echo "  make restart-api      - Restart the FastAPI backend"
	@echo "  make restart-engine   - Restart the Go trading engine"
	@echo "  make restart-frontend - Restart the React frontend"
	@echo "  make clean            - Stop all services AND remove database volumes (WIPES DATA)"
	@echo "  make install-frontend lib=<name> - Install a npm package in frontend"
	@echo "  make install-api lib=<name>      - Install a python package in api"

up:
	docker-compose up -d

down:
	docker-compose down

build:
	docker-compose up -d --build

logs:
	docker-compose logs -f

seed:
	docker exec omnimarket_api python seed_db.py

test-engine:
	docker run --rm -v "$(CURDIR)/backend/engine:/app" -w /app --network omni-market_default golang:1.26-alpine go test ./... -v -count=1

test-engine-race:
	docker run --rm -v "$(CURDIR)/backend/engine:/app" -w /app --network omni-market_default golang:1.26 go test ./... -v -race -count=1

benchmark-engine:
	docker run --rm -v "$(CURDIR)/backend/engine:/app" -w /app --network omni-market_default golang:1.26-alpine go test ./... -bench=. -run=^# -count=1

test-api:
	docker exec omnimarket_api pytest

test-frontend:
	npm run test --prefix frontend

test-all: test-engine test-api test-frontend

psql:
	docker exec -it omnimarket_db psql -U omnimarket -d omnimarket

restart-api:
	docker-compose restart api

restart-engine:
	docker-compose restart engine

restart-frontend:
	docker-compose restart frontend

clean:
	docker-compose down -v

install-frontend:
	docker exec omnimarket_frontend npm install $(lib)

install-api:
	docker exec omnimarket_api pip install $(lib)
	docker exec omnimarket_api pip freeze > requirements.txt
