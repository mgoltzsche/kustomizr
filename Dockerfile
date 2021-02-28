ARG KUSTOMIZE_VERSION=4.0.4

FROM golang:1.14-alpine3.12 AS build

RUN apk add --update --no-cache git curl
ARG KUSTOMIZE_VERSION
RUN curl -fsSL https://github.com/kubernetes-sigs/kustomize/releases/download/kustomize%2Fv${KUSTOMIZE_VERSION}/kustomize_v${KUSTOMIZE_VERSION}_linux_amd64.tar.gz | tar -xvzf - \
	 && mv kustomize /usr/local/bin/

ENV GO111MODULE=on CGO_ENABLED=0

COPY go.mod go.sum /go/src/github.com/mgoltzsche/kustomizr/
WORKDIR /go/src/github.com/mgoltzsche/kustomizr
RUN go mod download
COPY main.go /go/src/github.com/mgoltzsche/kustomizr/
RUN go build -ldflags '-s -w -extldflags "-static"' . && mv kustomizr /usr/local/bin/

FROM alpine:3.12
RUN apk add --update --no-cache git
ARG KUSTOMIZE_VERSION
ENV KUSTOMIZE_VERSION=${KUSTOMIZE_VERSION}
COPY --from=build /usr/local/bin/ /usr/local/bin/
ENTRYPOINT ["/usr/local/bin/kustomizr"]
