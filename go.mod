module github.com/mrincompetent/wireguard-controller

go 1.14

require (
	github.com/go-logr/zapr v0.1.1
	github.com/go-test/deep v1.0.5
	github.com/heptiolabs/healthcheck v0.0.0-20180807145615-6ff867650f40
	github.com/pmezard/go-difflib v1.0.1-0.20181226105442-5d4384ee4fb2
	github.com/prometheus/client_golang v1.5.1
	github.com/vishvananda/netlink v1.1.0
	go.uber.org/multierr v1.5.0
	go.uber.org/zap v1.15.0
	golang.org/x/crypto v0.0.0-20200414173820-0848c9571904 // indirect
	golang.org/x/sys v0.0.0-20200413165638-669c56c373c4 // indirect
	golang.zx2c4.com/wireguard v0.0.20200320 // indirect
	golang.zx2c4.com/wireguard/wgctrl v0.0.0-20200324154536-ceff61240acf
	gopkg.in/DATA-DOG/go-sqlmock.v1 v1.3.0 // indirect
	k8s.io/api v0.18.4
	k8s.io/apimachinery v0.18.4
	k8s.io/client-go v0.18.4
	sigs.k8s.io/controller-runtime v0.6.1
)
