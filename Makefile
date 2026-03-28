.PHONY: build run test clean deploy

build:
	go build -o bin/cv-api .

run:
	@set -a && . ./.env && set +a && go run .

test:
	go test ./...

clean:
	rm -rf bin/

deploy:
	flyctl deploy
