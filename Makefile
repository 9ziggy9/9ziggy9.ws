ifneq (,$(wildcard ./.env))
    include .env
    export
endif

PORT := $(PORT)

.PHONY: kill_server local docker_build docker

docker: docker_build
	docker run -p $(PORT):$(PORT) --env-file .env 9ziggy9.ws

docker_build:
	docker build --no-cache -t 9ziggy9.ws .

local: client.go servelog.go env.go main.go
	go run client.go servelog.go env.go main.go

kill_server:
	fuser -k -n tcp $(PORT)
