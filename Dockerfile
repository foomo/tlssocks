# -----------------------------------------------------------------------------
# Builder
# -----------------------------------------------------------------------------
FROM golang as builder

WORKDIR /service

COPY ./go.mod ./go.sum ./

RUN  go mod download

COPY . ./

ARG SERVICE_NAME=none

RUN go build -o /go/bin/service cmd/${SERVICE_NAME}/${SERVICE_NAME}.go

# -----------------------------------------------------------------------------
# Application
# -----------------------------------------------------------------------------
FROM scratch

COPY --from=builder /go/bin/service /usr/local/bin/service

ENTRYPOINT ["/usr/local/bin/service"]
