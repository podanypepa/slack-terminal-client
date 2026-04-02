BINARY := slack-terminal-client

.PHONY: build run lint check clean

build:
	go build -o $(BINARY) .

run: build
	./$(BINARY)

lint:
	gocritic check ./...

check:
	go vet ./...
	gocritic check ./...

clean:
	rm -f $(BINARY)
