FROM rust:1.87-bookworm AS builder
WORKDIR /src

COPY proto ./proto
COPY connector ./connector

RUN cd connector && cargo build --release

FROM debian:bookworm-slim
RUN apt-get update \
    && apt-get install -y --no-install-recommends ca-certificates \
    && rm -rf /var/lib/apt/lists/*

COPY --from=builder /src/connector/target/release/ztna-connector /usr/local/bin/ztna-connector

ENTRYPOINT ["ztna-connector"]
