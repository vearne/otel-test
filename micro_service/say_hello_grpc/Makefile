COMMAND_NAME = otel-sayhello-grpc

.PHONY: clean
clean:
	rm -rf $(COMMAND_NAME)

.PHONY: build
build: clean
	go build -o $(COMMAND_NAME) .

.PHONY: run
run:
	env OTEL_SERVICE_NAME=$(COMMAND_NAME) \
		OTEL_EXPORTER_OTLP_ENDPOINT=http://localhost:4317 \
		./$(COMMAND_NAME)