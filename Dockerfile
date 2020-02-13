# Builder image, produces a statically linked binary
FROM golang:1.13.4-alpine3.10 as node-build
RUN apk update && apk add libstdc++ g++ git

WORKDIR /src
ADD . ./

WORKDIR /src/cmd/dkgnode
RUN go build -mod=vendor -ldflags "-X github.com/torusresearch/torus-node/version.GitCommit=`git rev-list -1 HEAD`" 


# final image
FROM alpine:3.9
RUN apk update && apk add ca-certificates --no-cache

RUN mkdir -p /torus
COPY --from=node-build /src/cmd/dkgnode/dkgnode /torus/dkgnode

ENV GODEBUG madvdontneed=1

EXPOSE 80 443 1080 6060 8080 18080 26656 26657 26660
VOLUME ["/torus", "/root/https"]
CMD ["/torus/dkgnode"]
