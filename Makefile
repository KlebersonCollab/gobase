# Variables
BINARY_NAME=bin/server
DB_URL="postgres://postgres:postgres@localhost:5432/gobase?sslmode=disable"

.PHONY: all build run test up down clean help front-install front-dev front-build

all: front-build build

## up: Sobe o banco de dados temporário
up:
	docker compose up -d

## down: Derruba o banco de dados
down:
	docker compose down

## build: Compila o projeto Go
build:
	go build -o $(BINARY_NAME) cmd/gobase/main.go

## run: Executa o projeto Go com a URL do banco de dados
run:
	DATABASE_URL=$(DB_URL) go run cmd/gobase/main.go

## test: Executa os testes do Go
test:
	go test ./...

## front-install: Instala dependências do frontend
front-install:
	cd admin-ui && npm install

## front-dev: Inicia o servidor de desenvolvimento do frontend
front-dev:
	cd admin-ui && npm run dev

## front-build: Gera a build de produção do React
front-build:
	cd admin-ui && npm run build

## front-clean: Remove a build do frontend
front-clean:
	rm -rf admin-ui/dist

## clean: Remove binários e limpa cache (inclui frontend)
clean: front-clean
	rm -rf bin/
	go clean

## help: Mostra comandos disponíveis
help:
	@echo "Comandos disponíveis:"
	@sed -n 's/^##//p' $(MAKEFILE_LIST) | column -t -s ':' |  sed -e 's/^/ /'
