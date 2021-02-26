##############################
###### STAGE: BUILD     ######
##############################
FROM golang:alpine AS build-env

WORKDIR /src

COPY ./go.mod ./go.sum ./

# NOT SUPPORTED BY DOCKER HUB
RUN --mount=type=cache,target=/root/.cache/go-build \
    go mod download -x
# RUN go mod download

COPY ./ ./

# NOT SUPPORTED BY DOCKER HUB
RUN --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 go build -trimpath -o /tlssocks cmd/tlssocks/tlssocks.go cmd/tlssocks/authentication.go

##############################
###### STAGE: CONTAINER     ######
##############################
FROM alpine

COPY --from=build-env /tlssocks /usr/sbin/tlssocks

ENTRYPOINT [ "/usr/sbin/tlssocks" ]

CMD ["/usr/sbin/tlssocks"]
