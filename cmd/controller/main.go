package main

import (
	"flag"
	"net"

	cniconfig "github.com/mrincompetent/wireguard-controller/pkg/controller/cni-config"
	"github.com/mrincompetent/wireguard-controller/pkg/controller/node"
	"github.com/mrincompetent/wireguard-controller/pkg/controller/route"
	"github.com/mrincompetent/wireguard-controller/pkg/controller/telemetry"
	wireguard_interface "github.com/mrincompetent/wireguard-controller/pkg/controller/wireguard-interface"
	"github.com/mrincompetent/wireguard-controller/pkg/wireguard/key"

	"github.com/go-logr/zapr"
	"go.uber.org/zap"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
	ctrl "sigs.k8s.io/controller-runtime"
	ctrlzap "sigs.k8s.io/controller-runtime/pkg/log/zap"
)

var (
	interfaceName          = flag.String("interface", "wg-kube", "Name of the WireGuard interface to use")
	nodeName               = flag.String("node-name", "", "Name of the node this pod is running on")
	privateKeyPath         = flag.String("private-key", "/etc/wireguard/wg-kube-key", "Path to the private key for WireGuard")
	cniTargetDir           = flag.String("cni-config-path", "/etc/cni/net.d/", "Path where the CNI configs should be written to")
	cniSourceDir           = flag.String("cni-tpl-path", "/cni-tpl/", "Path where the CNI config templates are stored")
	podCIDR                = flag.String("pod-cidr", "", "Pod CIDR")
	wireGuardPort          = flag.Int("wireguard-port", 51820, "WireGuard listening port")
	telemetryListenAddress = flag.String("telemetry-listen-address", "127.0.0.1:8080", "Listen address for the telemetry http server")
	development            = flag.Bool("development", false, "enable development logging")
)

func main() {
	flag.Parse()

	log := ctrlzap.NewRaw(enableDevelopment(*development))
	defer log.Sync()
	ctrl.SetLogger(zapr.NewLogger(log))

	if *podCIDR == "" {
		log.Panic("pod-cidr must be set")
	}

	_, podCidrNet, err := net.ParseCIDR(*podCIDR)
	if err != nil {
		log.Panic("unable to parse pod cidr", zap.Error(err))
	}

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		// Disable the integrated listener
		// We have our own which also exposes pprof & health endpoints
		MetricsBindAddress: "0",
	})
	if err != nil {
		log.Panic("Unable to start manager", zap.Error(err))
	}

	// We pass in a func to load the key as we generate it in a dedicated controller.
	// Also we handle the not found error here to avoid importing a new dependency
	loadKey := func() (*wgtypes.Key, bool, error) {
		k, err := key.LoadPrivateKey(*privateKeyPath)
		if err != nil {
			if key.IsPrivateKeyNotFound(err) {
				return nil, false, nil
			}

			return nil, false, err
		}

		return k, true, nil
	}

	if err := wireguard_interface.Add(
		mgr,
		log,
		*interfaceName,
		*wireGuardPort,
		*nodeName,
		loadKey,
	); err != nil {
		log.Panic("Unable to add the WireGuard interface controller to the controller manager", zap.Error(err))
	}

	if err := cniconfig.Add(
		mgr,
		log,
		*cniSourceDir,
		*cniTargetDir,
		*interfaceName,
		podCidrNet,
		*nodeName,
	); err != nil {
		log.Panic("Unable to add the cni config controller to the controller manager", zap.Error(err))
	}

	if err := route.Add(
		mgr,
		log,
		*interfaceName,
		*nodeName,
	); err != nil {
		log.Panic("Unable to add the route controller to the controller manager", zap.Error(err))
	}

	if err := node.Add(
		mgr,
		log,
		*nodeName,
		*privateKeyPath,
		*wireGuardPort,
	); err != nil {
		log.Panic("Unable to add the node controller to the controller manager", zap.Error(err))
	}

	if err := telemetry.Add(
		mgr,
		log,
		*telemetryListenAddress,
	); err != nil {
		log.Panic("Unable to add the telemetry server to the controller manager", zap.Error(err))
	}

	log.Info("Starting manager")

	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		log.Panic("problem running manager", zap.Error(err))
	}
}

func enableDevelopment(b bool) func(o *ctrlzap.Options) {
	return func(o *ctrlzap.Options) {
		o.Development = b
	}
}
