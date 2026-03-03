.PHONY: proto proto-compat proto-baseline fmt db-up db-down db-logs tls-dev-cert controller-run connector-run agent-run admin-api-run admin-web-run docker-connector-image docker-agent-image docker-images

TLS_DEV_CN ?= localhost
TLS_DEV_SAN ?= DNS:localhost,IP:127.0.0.1
WORKSPACE_ID ?= demo
BOOTSTRAP_TOKEN ?= demo
CONTROLLER_ADDR ?= https://127.0.0.1:8443
CONTROLLER_CA_CERT ?= ../deploy/tls/controller.crt
CONNECTOR_DATAPLANE_LISTEN_ADDR ?= 0.0.0.0:9443
AGENT_STORAGE_DIR ?= /var/lib/ztna/agent
AGENT_CONNECTOR_ADDR ?=
AGENT_CONNECTOR_SERVER_NAME ?=
AGENT_DATAPLANE_PING_INTERVAL_SECS ?= 15
CONNECTOR_IMAGE ?= ztna/connector:local
AGENT_IMAGE ?= ztna/agent:local

proto:
	buf generate

proto-compat:
	buf breaking proto --against proto/breaking/baseline.binpb

proto-baseline:
	buf build proto -o proto/breaking/baseline.binpb

fmt:
	gofmt -w $$(find controller -name '*.go' -type f)
	cd connector && cargo fmt
	cd agent && cargo fmt

db-up:
	docker compose up -d postgres

db-down:
	docker compose down

db-logs:
	docker compose logs -f postgres

tls-dev-cert:
	mkdir -p deploy/tls
	openssl req -x509 -newkey rsa:2048 -sha256 -days 365 -nodes \
		-keyout deploy/tls/controller.key \
		-out deploy/tls/controller.crt \
		-subj "/CN=$(TLS_DEV_CN)" \
		-addext "basicConstraints=critical,CA:FALSE" \
		-addext "keyUsage=critical,digitalSignature,keyEncipherment" \
		-addext "extendedKeyUsage=serverAuth" \
		-addext "subjectAltName=$(TLS_DEV_SAN)"

controller-run:
	cd controller && go run ./cmd/controller

connector-run:
	cd connector && cargo run -- \
		--workspace-id $(WORKSPACE_ID) \
		--bootstrap-token $(BOOTSTRAP_TOKEN) \
		--controller-addr $(CONTROLLER_ADDR) \
		--controller-ca-cert $(CONTROLLER_CA_CERT) \
		--dataplane-listen-addr $(CONNECTOR_DATAPLANE_LISTEN_ADDR)

agent-run:
	cd agent && cargo run -- \
		--workspace-id $(WORKSPACE_ID) \
		--bootstrap-token $(BOOTSTRAP_TOKEN) \
		--controller-addr $(CONTROLLER_ADDR) \
		--controller-ca-cert $(CONTROLLER_CA_CERT) \
		--storage-dir $(AGENT_STORAGE_DIR) \
		--connector-addr $(AGENT_CONNECTOR_ADDR) \
		--connector-server-name $(AGENT_CONNECTOR_SERVER_NAME) \
		--dataplane-ping-interval-secs $(AGENT_DATAPLANE_PING_INTERVAL_SECS)

admin-api-run:
	cd admin-api && npm run dev

admin-web-run:
	cd admin-web && npm run dev

docker-connector-image:
	docker build -f deploy/docker/connector.Dockerfile -t $(CONNECTOR_IMAGE) .

docker-agent-image:
	docker build -f deploy/docker/agent.Dockerfile -t $(AGENT_IMAGE) .

docker-images: docker-connector-image docker-agent-image

# Release targets for building binaries locally
RELEASE_DIR ?= ./dist
RELEASE_VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")

release-prep:
	mkdir -p $(RELEASE_DIR)

release-controller-linux-amd64: release-prep
	cd controller && GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -ldflags="-s -w -X main.version=$(RELEASE_VERSION)" -o ../$(RELEASE_DIR)/ztna-controller-linux-amd64 ./cmd/controller

release-controller-linux-arm64: release-prep
	cd controller && GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build -ldflags="-s -w -X main.version=$(RELEASE_VERSION)" -o ../$(RELEASE_DIR)/ztna-controller-linux-arm64 ./cmd/controller

release-controller-darwin-amd64: release-prep
	cd controller && GOOS=darwin GOARCH=amd64 CGO_ENABLED=0 go build -ldflags="-s -w -X main.version=$(RELEASE_VERSION)" -o ../$(RELEASE_DIR)/ztna-controller-darwin-amd64 ./cmd/controller

release-controller-darwin-arm64: release-prep
	cd controller && GOOS=darwin GOARCH=arm64 CGO_ENABLED=0 go build -ldflags="-s -w -X main.version=$(RELEASE_VERSION)" -o ../$(RELEASE_DIR)/ztna-controller-darwin-arm64 ./cmd/controller

release-connector-linux-amd64: release-prep
	cd connector && cargo build --release --target x86_64-unknown-linux-gnu && cp target/x86_64-unknown-linux-gnu/release/ztna-connector ../$(RELEASE_DIR)/ztna-connector-linux-amd64

release-connector-linux-arm64: release-prep
	cd connector && cross build --release --target aarch64-unknown-linux-gnu && cp target/aarch64-unknown-linux-gnu/release/ztna-connector ../$(RELEASE_DIR)/ztna-connector-linux-arm64

release-connector-darwin-amd64: release-prep
	cd connector && cargo build --release --target x86_64-apple-darwin && cp target/x86_64-apple-darwin/release/ztna-connector ../$(RELEASE_DIR)/ztna-connector-darwin-amd64

release-connector-darwin-arm64: release-prep
	cd connector && cargo build --release --target aarch64-apple-darwin && cp target/aarch64-apple-darwin/release/ztna-connector ../$(RELEASE_DIR)/ztna-connector-darwin-arm64

release-agent-linux-amd64: release-prep
	cd agent && cargo build --release --target x86_64-unknown-linux-gnu && cp target/x86_64-unknown-linux-gnu/release/ztna-agent ../$(RELEASE_DIR)/ztna-agent-linux-amd64

release-agent-linux-arm64: release-prep
	cd agent && cross build --release --target aarch64-unknown-linux-gnu && cp target/aarch64-unknown-linux-gnu/release/ztna-agent ../$(RELEASE_DIR)/ztna-agent-linux-arm64

release-agent-darwin-amd64: release-prep
	cd agent && cargo build --release --target x86_64-apple-darwin && cp target/x86_64-apple-darwin/release/ztna-agent ../$(RELEASE_DIR)/ztna-agent-darwin-amd64

release-agent-darwin-arm64: release-prep
	cd agent && cargo build --release --target aarch64-apple-darwin && cp target/aarch64-apple-darwin/release/ztna-agent ../$(RELEASE_DIR)/ztna-agent-darwin-arm64

release-all: release-controller-linux-amd64 release-controller-linux-arm64 release-controller-darwin-amd64 release-controller-darwin-arm64 release-connector-linux-amd64 release-connector-linux-arm64 release-connector-darwin-amd64 release-connector-darwin-arm64 release-agent-linux-amd64 release-agent-linux-arm64 release-agent-darwin-amd64 release-agent-darwin-arm64
	@echo "All binaries built in $(RELEASE_DIR)/"
	@chmod +x $(RELEASE_DIR)/*
	@cd $(RELEASE_DIR) && sha256sum * > checksums.txt
	@echo "Checksums created: $(RELEASE_DIR)/checksums.txt"

release-checksums:
	cd $(RELEASE_DIR) && sha256sum * > checksums.txt

release-clean:
	rm -rf $(RELEASE_DIR)

release-list:
	@ls -la $(RELEASE_DIR)/ 2>/dev/null || echo "No release directory found. Run 'make release-all' first."
