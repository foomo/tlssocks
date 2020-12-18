##############################
###### STAGE: BUILD     ######
##############################
FROM golang:latest AS build-env

WORKDIR /src

COPY ./go.mod ./go.sum ./

RUN --mount=type=cache,target=/root/.cache/go-build \
    go mod download

COPY ./ ./

RUN --mount=type=cache,target=/root/.cache/go-build \
    go build -trimpath -o /tlssocksproxy cmd/tlssocksproxy/tlssocksproxy.go

##############################
###### STAGE: CONTAINER     ######
##############################
FROM alpine

COPY --from=build-env tlssocksproxy /usr/sbin/tlssocksproxy

ENTRYPOINT [ "/usr/sbin/tlssocksproxy" ]