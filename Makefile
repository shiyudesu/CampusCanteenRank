.PHONY: test-backend dev-up dev-down logs api-run

test-backend:
	go test ./server/...

dev-up:
	docker compose --env-file .env.example up -d --build

dev-down:
	docker compose --env-file .env.example down

logs:
	docker compose --env-file .env.example logs -f api

api-run:
	go run ./server/cmd/api
