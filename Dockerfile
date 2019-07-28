# ------------------------------------------------------------------------------
# Build image
# ------------------------------------------------------------------------------
FROM golang:1.12.7 as build_img

RUN apt-get update && apt-get install -y jq bash curl git

ADD . $GOPATH/src/github.com/mrincompetent/wireguard-controller/

RUN mkdir -p /cni-bin && \
  cd /cni-bin && \
  curl -L https://github.com/containernetworking/plugins/releases/download/v0.7.5/cni-plugins-amd64-v0.7.5.tgz | tar -xvz

ENV GO111MODULE=on
ENV CGO_ENABLED=0
RUN cd $GOPATH/src/github.com/mrincompetent/wireguard-controller/ && \
  go mod tidy && \
  go build -o /wireguard-controller -ldflags "-s" -a -installsuffix cgo github.com/mrincompetent/wireguard-controller/cmd/controller

# ------------------------------------------------------------------------------
# App image
# ------------------------------------------------------------------------------
FROM alpine:3.9 as prod_img
COPY --from=build_img /wireguard-controller /wireguard-controller
COPY --from=build_img /cni-bin /cni-bin

COPY tpl/10-bridge.conflist /cni-tpl/
