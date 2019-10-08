module github.com/mrincompetent/wireguard-controller

go 1.13

require (
	github.com/go-logr/zapr v0.1.1
	github.com/go-test/deep v1.0.2-0.20181118220953-042da051cf31
	github.com/heptiolabs/healthcheck v0.0.0-20180807145615-6ff867650f40
	github.com/pkg/errors v0.8.1
	github.com/pmezard/go-difflib v1.0.1-0.20181226105442-5d4384ee4fb2
	github.com/prometheus/client_golang v0.9.0
	github.com/vishvananda/netlink v1.0.1-0.20190608042107-0f040b9e2cdf
	github.com/vishvananda/netns v0.0.0-20180720170159-13995c7128cc // indirect
	go.uber.org/multierr v1.1.0
	go.uber.org/zap v1.9.1
	golang.org/x/net v0.0.0-20190620200207-3b0461eec859 // indirect
	golang.zx2c4.com/wireguard/wgctrl v0.0.0-20190607034155-226bf4e412cd
	k8s.io/api v0.0.0-20190409021203-6e4e0e4f393b
	k8s.io/apimachinery v0.0.0-20190404173353-6a84e37a896d
	k8s.io/client-go v11.0.1-0.20190409021438-1a26190bd76a+incompatible
	sigs.k8s.io/controller-runtime v0.2.2
)
