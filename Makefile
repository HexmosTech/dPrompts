APP = dprompts
SRC = .

build:
	go build -o $(APP) $(SRC)

client:
	go run . --mode=client --args='{"prompt":"example prompt"}' --metadata='{"type":"example"}'

worker:
	go run . --mode=worker

.PHONY: build client worker
