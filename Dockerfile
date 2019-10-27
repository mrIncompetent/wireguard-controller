FROM alpine:3.10.3

RUN mkdir -p /cni-bin && \
    wget -O cni-plugins.tgz https://github.com/containernetworking/plugins/releases/download/v0.8.2/cni-plugins-linux-amd64-v0.8.2.tgz && \
    tar -xzf cni-plugins.tgz -C /cni-bin && \
    rm cni-plugins.tgz

ADD ./wireguard-controller /wireguard-controller