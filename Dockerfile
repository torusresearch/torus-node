# Builder image, produces a statically linked binary
FROM golang:1.12.1-alpine3.9 as node-build


RUN apk update && apk add bash make git gcc libstdc++ g++ musl-dev
RUN apk add --no-cache \
    --repository http://nl.alpinelinux.org/alpine/edge/testing \
    leveldb-dev

WORKDIR /src
ADD go.mod go.sum ./
RUN go mod download

ADD . ./

WORKDIR /src/cmd/dkgnode
RUN go build


# final image
FROM alpine:3.9

RUN apk update && apk add ca-certificates --no-cache
RUN apk add --no-cache \
  --repository http://nl.alpinelinux.org/alpine/edge/testing \
  leveldb

RUN mkdir -p /torus

COPY --from=node-build /src/cmd/dkgnode/dkgnode /torus/dkgnode

EXPOSE 443 80 1080 26656 26657
VOLUME ["/torus", "/root/https"]
CMD ["/torus/dkgnode"]
