FROM rust:latest as BUILDER
LABEL maintainer="chain@crypto.org"

RUN apt-get update && \
    apt-get -y install protobuf-compiler && \
    cd /tmp && \
    git clone https://github.com/crypto-com/ledger-rs.git && \
    cd ledger-rs && \
    git checkout -b crypto abba8c6cb31dc81b89e24a0132be101432b994b5 && \
    cd examples/zemu-grpc-server && \
    cargo build --release


FROM zondax/builder-zemu@sha256:4b793ac77c29870e6046e1d0a5019643fd178530205f9cf983bfadd114abca0a
COPY ./app.elf /home/zondax/speculos/apps/crypto.elf
COPY --from=BUILDER /tmp/ledger-rs/examples/zemu-grpc-server/target/release/zemu-grpc-server /usr/local/bin
ENTRYPOINT []

