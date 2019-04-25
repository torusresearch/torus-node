# Builder image, produces a statically linked binary
FROM golang:1.12.1-alpine3.9 as node-build


RUN apk update && apk add bash make git gcc libstdc++ g++ musl-dev
RUN apk add --no-cache \
    --repository http://nl.alpinelinux.org/alpine/edge/community \
    leveldb-dev
RUN apk update && apk add bash make git gcc libstdc++ g++ musl-dev
RUN apk update && apk add ca-certificates --no-cache
RUN apk add --no-cache \
  --repository http://nl.alpinelinux.org/alpine/edge/community \
  leveldb

# add delve debugger
RUN go get -u github.com/go-delve/delve/cmd/dlv


WORKDIR /src
RUN mkdir -p /torus
ADD . ./torus

# test
# RUN go test -mod=vendor -cover ./dkgnode ./logging ./pvss ./common ./tmabci

WORKDIR /src/torus/cmd/dkgnode
RUN go build -mod=vendor

EXPOSE 443 80 1080 26656 26657 40000

VOLUME ["/src/torus", "/root/https"]
CMD ["dlv", "exec", "/src/torus/cmd/dkgnode/dkgnode", "--listen=:40000", "--headless=true", "--api-version=2", "--log"]

# dlv exec /src/torus/cmd/dkgnode --listen=:40000 --headless=true --api-version=2 --log

# final image
# FROM golang:1.12.1-alpine3.9
# WORKDIR /
# RUN apk update && apk add bash make git gcc libstdc++ g++ musl-dev
# RUN apk update && apk add ca-certificates --no-cache
# RUN apk add --no-cache \
#   --repository http://nl.alpinelinux.org/alpine/edge/community \
#   leveldb


# RUN mkdir -p /src/torus

# COPY --from=node-build /src/torus/ /src/torus/
# COPY --from=node-build /go/bin/dlv /go/bin/dlv

# EXPOSE 443 80 1080 26656 26657 40000
# VOLUME ["/src/torus", "/root/https"]

# # setup remote debugging
# WORKDIR /src/torus/cmd/dkgnode
# CMD [ "dlv", "debug", "--listen=:40000", "--headless=true", "--api-version=2", "--log" ]

# CMD ["/torus/dkgnode"]
