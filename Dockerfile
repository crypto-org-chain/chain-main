
# Simple usage with a mounted data directory:
# > docker build -t cryptocom/chain-main .
# > docker run -it -p 26657:26657 -p 26656:26656 -v ~/.chainmaind:/chain-main/.chainmaind -v ~/.chainmaincli:/chain-main/.chainmaincli cryptocom/chain-main chain-maind init [moniker] [flags]
# > docker run -it -p 26657:26657 -p 26656:26656 -v ~/.chainmaind:/chain-main/.chainmaind -v ~/.chainmaincli:/chain-main/.chainmaincli cryptocom/chain-main chain-maind start
FROM golang:alpine AS build-env

# Set up dependencies
ENV PACKAGES curl make git libc-dev bash gcc linux-headers eudev-dev python3

# Set working directory for the build
WORKDIR /go/src/github.com/crypto-com/chain-main

# Add source files
COPY . .

# Install minimum necessary dependencies, build Cosmos SDK, remove packages
RUN apk add --no-cache $PACKAGES && \
  make install

# Final image
FROM alpine:edge

ENV CHAIN_MAIN /chain-main

# Install ca-certificates
RUN apk add --update ca-certificates

RUN addgroup chain-main && \
  adduser -S -G chain-main chain-main -h "$CHAIN_MAIN"

USER chain-main

WORKDIR $CHAIN_MAIN

# Copy over binaries from the build-env
COPY --from=build-env /go/bin/chain-maind /usr/bin/chain-maind
COPY --from=build-env /go/bin/chain-maincli /usr/bin/chain-maincli

# Run chain-maind by default, omit entrypoint to ease using container with chain-maincli
CMD ["chain-maind"]