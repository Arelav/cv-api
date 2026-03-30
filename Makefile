.PHONY: build run test clean deploy

build:
	go build -o bin/cv-api .

# Use 1Password CLI to resolve op://… in .env (plain `source .env` does not).
run:
	op run --env-file=.env -- go run .

test:
	go test ./...

clean:
	rm -rf bin/

deploy:
	flyctl deploy
