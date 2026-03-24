.PHONY: test-backend dev-up dev-down logs api-run

test-backend:
	go test ./server/...

test-backend-integration:
	@echo "Running backend integration tests with MYSQL_DSN/REDIS_ADDR"
	@test -n "$(MYSQL_DSN)" || (echo "MYSQL_DSN is required" && exit 1)
	@test -n "$(REDIS_ADDR)" || (echo "REDIS_ADDR is required" && exit 1)
	MYSQL_DSN="$(MYSQL_DSN)" REDIS_ADDR="$(REDIS_ADDR)" go test ./server/...

dev-up:
	docker compose --env-file .env.example up -d --build

dev-down:
	docker compose --env-file .env.example down

logs:
	docker compose --env-file .env.example logs -f api

api-run:
	go run ./server/cmd/api
