.PHONY: build run test clean deploy

build:
	go build -o bin/cv-api .

run:
	go run .

test:
	go test ./...

clean:
	rm -rf bin/

deploy:
	flyctl deploy
