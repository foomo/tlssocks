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
# RUN go mod download

COPY ./ ./

# CGO_ENABLED=1 go build -race ...
# RUN apk add build-base

# NOT SUPPORTED BY DOCKER HUB
RUN --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 go build -trimpath -o /tlssocksproxy cmd/tlssocksproxy/tlssocksproxy.go

# RUN CGO_ENABLED=0 go build -trimpath -o /tlssocksproxy cmd/tlssocksproxy/tlssocksproxy.go

##############################
###### STAGE: CONTAINER     ######
##############################
FROM alpine

COPY --from=build-env /tlssocksproxy /usr/sbin/tlssocksproxy

ENTRYPOINT [ "/usr/sbin/tlssocksproxy" ]

CMD ["/usr/sbin/tlssocksproxy"]
