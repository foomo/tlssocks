##############################
###### STAGE: BUILD     ######
##############################
FROM golang:alpine AS build-env

WORKDIR /src

COPY ./go.mod ./go.sum ./

# NOT SUPPORTED BY DOCKER HUB
RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg/mod \
    go mod download -x

COPY ./ ./

# NOT SUPPORTED BY DOCKER HUB
RUN --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 go build -trimpath -o /tcpproxy cmd/tcpproxy/tcpproxy.go

##############################
###### STAGE: CONTAINER     ######
##############################
FROM alpine

COPY --from=build-env /tcpproxy /usr/sbin/tcpproxy

ENTRYPOINT [ "/usr/sbin/tcpproxy" ]

CMD ["/usr/sbin/tlssocks"]
