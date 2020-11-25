FROM alpine:3.12.1

RUN mkdir -p /cni-bin && \
    wget -O cni-plugins.tgz https://github.com/containernetworking/plugins/releases/download/v0.8.7/cni-plugins-linux-amd64-v0.8.7.tgz && \
    tar -xzf cni-plugins.tgz -C /cni-bin && \
    rm cni-plugins.tgz

ADD ./wireguard-controller /wireguard-controller
