
BIN_NAME	:= mailmover
IMAGE_NAME	:= mailmover
TAG 		?= $(shell git rev-parse --short HEAD)
BUILD_DIR	:= build
BIN_PATH	:= $(BUILD_DIR)/$(BIN_NAME)
REGISTRY	:= registry.example.com/personal

.PHONY: all
all: build-image

$(BIN_PATH): $(shell find . -name '*.go') go.mod go.sum
	mkdir -p $(BUILD_DIR)
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -ldflags="-X main.buildSHA=$(TAG)" -o $(BIN_PATH) .

.PHONEY: build-image
build-image: $(BIN_PATH)
	docker build \
		--build-arg BIN_PATH=$(BIN_PATH) \
		-t $(IMAGE_NAME):$(TAG) \
		-t $(IMAGE_NAME):latest .

.PHONEY: push-image
push-image: build-image
	docker tag $(IMAGE_NAME):$(TAG) $(REGISTRY)/$(IMAGE_NAME):$(TAG)
	docker tag $(IMAGE_NAME):$(TAG) $(REGISTRY)/$(IMAGE_NAME):latest
	docker push $(REGISTRY)/$(IMAGE_NAME):$(TAG)
	docker push $(REGISTRY)/$(IMAGE_NAME):latest

.PHONEY: deploy
deploy: push-image
	kubectl apply -f manifest.yaml

.PHONY: clean
clean:
	rm -f mailmover
