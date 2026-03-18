BINARY_NAME=agent
IMG ?= cowell-images.tencentcloudcr.com/app-test/compute-sentry-agent:latest

.PHONY: build
build:
	go build -o bin/$(BINARY_NAME) main.go

.PHONY: docker-build
docker-build:
	docker build -t $(IMG) -f Dockerfile ..

.PHONY: docker-push
docker-push:
	docker push $(IMG)

.PHONY: clean
clean:
	rm -rf bin/
