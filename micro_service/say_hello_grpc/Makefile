COMMAND_NAME = sw-sayhello-grpc

.PHONY: clean
clean:
	rm -rf $(COMMAND_NAME)

.PHONY: build
build:
	go build -mod=vendor -toolexec="/var/tmp/skywalking-go-agent" -work -a -o $(COMMAND_NAME) .

.PHONY: run
run:
	env SW_AGENT_NAME=$(COMMAND_NAME)  \
		CONSUL_DISCOVER_DC_LIST=baoding \
		SW_AGENT_REPORTER_GRPC_BACKEND_SERVICE=consul://consul-cache.online.qiyi.qae/skywalking-oap-server \
		./$(COMMAND_NAME)