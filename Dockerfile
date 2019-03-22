# Builder image, produces a statically linked binary
FROM golang:1.11.2-alpine AS node-build


RUN apk update && apk add bash make git gcc libstdc++ g++ musl-dev
RUN apk add --no-cache \
    --repository http://nl.alpinelinux.org/alpine/edge/testing \
    leveldb-dev
RUN mkdir -p /go/src/github.com/tendermint/tendermint && \
    git clone https://github.com/YZhenY/tendermint /go/src/github.com/tendermint/tendermint

WORKDIR /go/src/github.com/tendermint/tendermint
RUN make get_tools && make get_vendor_deps


COPY . /go/src/github.com/torusresearch/torus-public
WORKDIR /go/src/github.com/torusresearch/torus-public/cmd/dkgnode

RUN go build

# final image
FROM alpine:3.7

RUN apk update && apk add ca-certificates --no-cache
RUN apk add --no-cache \
  --repository http://nl.alpinelinux.org/alpine/edge/testing \
  leveldb

RUN mkdir -p /torus

COPY --from=node-build /go/src/github.com/torusresearch/torus-public/cmd/dkgnode/dkgnode /torus/dkgnode

EXPOSE 443 80 1080 26656 26657
VOLUME ["/torus", "/root/https"]
CMD ["/torus/dkgnode"]
