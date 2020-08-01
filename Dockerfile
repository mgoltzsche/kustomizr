FROM golang:1.14-alpine3.12 AS build

RUN apk add --update --no-cache git curl
RUN curl -fsSL https://github.com/kubernetes-sigs/kustomize/releases/download/kustomize%2Fv3.8.1/kustomize_v3.8.1_linux_amd64.tar.gz | tar -xvzf - && mv kustomize /usr/local/bin/

ENV GO111MODULE=on CGO_ENABLED=0

COPY go.mod go.sum /go/src/github.com/mgoltzsche/kpt-kustomize/
WORKDIR /go/src/github.com/mgoltzsche/kpt-kustomize
RUN go mod download
COPY main.go /go/src/github.com/mgoltzsche/kpt-kustomize/
RUN go build -ldflags '-s -w -extldflags "-static"' . && mv kpt-kustomize /usr/local/bin/

FROM alpine:3.12
RUN apk add --update --no-cache git
COPY --from=build /usr/local/bin/ /usr/local/bin/
ENTRYPOINT ["/usr/local/bin/kpt-kustomize"]
