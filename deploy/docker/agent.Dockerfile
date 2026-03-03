FROM rust:1.87-bookworm AS builder
WORKDIR /src

COPY proto ./proto
COPY agent ./agent

RUN cd agent && cargo build --release

FROM debian:bookworm-slim
RUN apt-get update \
    && apt-get install -y --no-install-recommends ca-certificates \
    && rm -rf /var/lib/apt/lists/*

COPY --from=builder /src/agent/target/release/ztna-agent /usr/local/bin/ztna-agent

ENTRYPOINT ["ztna-agent"]
