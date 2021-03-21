BINARY_NAME=listener

all: deps build
install:
	go install listener.go
build:
	go build -o $(BINARY_NAME) listener.go
static-build:
	CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o ${BINARY_NAME} .
clean:
	go clean
	rm -f $(BINARY_NAME)
upgrade:
	go get -u