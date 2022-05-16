BUILD_DIR = build
PRODUCER_OPTS = GOOS=linux GOARCH=arm GOARM=6
CONSUMER_OPTS = GOOS=linux GOARCH=amd64

all: build_producer build_consumer

deploy: deploy_producer deploy_consumer

clean:
	rm -rf $(BUILD_DIR)/

build_dir:
	@mkdir -p $(BUILD_DIR)/

# Producer Targets #

build_producer: build_dir
	$(PRODUCER_OPTS) go build -o $(BUILD_DIR)/ ./src/tsproducer

deploy_producer: build_producer
ifndef PRODUCER_HOST
	$(error PRODUCER_HOST is not set)
endif
	scp $(BUILD_DIR)/tsproducer $(PRODUCER_HOST):~

# Consumer Targets #

build_consumer: build_dir
	$(CONSUMER_OPTS) go build -o $(BUILD_DIR)/ ./src/tsconsumer

deploy_consumer: build_consumer
ifndef CONSUMER_HOST
	$(error CONSUMER_HOST is not set)
endif
	scp $(BUILD_DIR)/tsconsumer $(CONSUMER_HOST):~
