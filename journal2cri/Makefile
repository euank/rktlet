.PHONY: all clean

TAG="0.0.1"
SOURCES := $(shell find ./journal2cri ./cmd -name '*.go')

all: ./bin/journal2cri

./bin/journal2cri: $(SOURCES)
	go build -o bin/journal2cri ./cmd/journal2cri/main.go

clean:
	rm -f ./bin/journal2cri

docker: ./bin/journal2cri
	docker build -t quay.io/coreos/rktlet-journal2cri:$(TAG) .

docker-push: docker
	docker push quay.io/coreos/rktlet-journal2cri:$(TAG)
