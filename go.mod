module github.com/mrincompetent/wireguard-controller

go 1.12

require (
	github.com/alecthomas/template v0.0.0-20190718012654-fb15b899a751 // indirect
	github.com/alecthomas/units v0.0.0-20190717042225-c3de453c63f4 // indirect
	github.com/go-logr/zapr v0.1.1
	github.com/go-test/deep v1.0.2-0.20181118220953-042da051cf31
	github.com/heptiolabs/healthcheck v0.0.0-20180807145615-6ff867650f40
	github.com/pkg/errors v0.8.1
	github.com/pmezard/go-difflib v1.0.1-0.20181226105442-5d4384ee4fb2
	github.com/prometheus/client_golang v0.9.0
	github.com/prometheus/common v0.0.0-20180801064454-c7de2306084e
	github.com/sirupsen/logrus v1.4.2 // indirect
	github.com/vishvananda/netlink v1.0.1-0.20190608042107-0f040b9e2cdf
	github.com/vishvananda/netns v0.0.0-20180720170159-13995c7128cc // indirect
	go.uber.org/multierr v1.1.0
	go.uber.org/zap v1.9.1
	golang.org/x/tools v0.0.0-20190829210313-340205e581e5 // indirect
	golang.zx2c4.com/wireguard/wgctrl v0.0.0-20190607034155-226bf4e412cd
	gopkg.in/alecthomas/kingpin.v2 v2.2.6 // indirect
	k8s.io/api v0.0.0-20190409021203-6e4e0e4f393b
	k8s.io/apimachinery v0.0.0-20190404173353-6a84e37a896d
	k8s.io/client-go v11.0.1-0.20190409021438-1a26190bd76a+incompatible
	sigs.k8s.io/controller-runtime v0.2.0-rc.0
)
