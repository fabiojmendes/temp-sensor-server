BUILD_DIR = build
PRODUCER_OPTS = GOOS=linux GOARCH=arm GOARM=6
CONSUMER_OPTS = GOOS=linux

all: tsproducer tsconsumer

clean:
	rm -rf $(BUILD_DIR)/

build_dir:
	@mkdir -p $(BUILD_DIR)/

tsproducer: build_producer deploy_producer

tsconsumer: build_consumer deploy_consumer

build_producer: build_dir
	$(PRODUCER_OPTS) go build -o $(BUILD_DIR)/ ./src/tsproducer

deploy_producer:
	scp $(BUILD_DIR)/tsproducer zero.local:~

build_consumer: build_dir
	$(CONSUMER_OPTS) go build -o $(BUILD_DIR)/ ./src/tsconsumer

deploy_consumer:
	scp $(BUILD_DIR)/tsconsumer linux-desktop.local:~
