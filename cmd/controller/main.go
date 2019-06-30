package main

import (
	"context"
	"flag"
	"fmt"
	"io/ioutil"
	"net"
	"os"

	"github.com/mrincompetent/wireguard-controller/pkg/controller/cni-config"
	"github.com/mrincompetent/wireguard-controller/pkg/controller/route"
	"github.com/mrincompetent/wireguard-controller/pkg/controller/telemetry"
	"github.com/mrincompetent/wireguard-controller/pkg/controller/wireguard-interface"
	"github.com/mrincompetent/wireguard-controller/pkg/kubernetes"
	customlog "github.com/mrincompetent/wireguard-controller/pkg/log"

	"github.com/go-logr/zapr"
	"github.com/pkg/errors"
	"github.com/vishvananda/netlink"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"golang.zx2c4.com/wireguard/wgctrl"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	logFormat              string
	interfaceName          string
	nodeName               string
	privateKeyPath         string
	cniTargetDir           string
	cniSourceDir           string
	podCIDR                string
	listeningPort          int
	telemetryListenAddress string
)

func main() {
	flag.StringVar(&interfaceName, "interface", "wg-kube", "Name of the WireGuard interface to use")
	flag.StringVar(&nodeName, "node-name", "", "Name of the node this pod is running on")
	flag.StringVar(&privateKeyPath, "private-key", "/etc/wireguard/wg-kube-key", "Path to the private key for WireGuard")
	flag.StringVar(&cniTargetDir, "cni-config-path", "/etc/cni/net.d/", "Path where the CNI configs should be written to")
	flag.StringVar(&cniSourceDir, "cni-tpl-path", "/cni-tpl/", "Path where the CNI config templates are stored")
	flag.StringVar(&podCIDR, "pod-cidr", "", "Pod CIDR")
	flag.StringVar(&telemetryListenAddress, "telemetry-listen-address", "127.0.0.1:8080", "Listen address for the telemetry http server")
	flag.IntVar(&listeningPort, "wireguard-port", 51820, "WireGuard listening port")
	flag.StringVar(&logFormat, "log-format", customlog.FormatJSON, "Log format")
	logLevel := zap.LevelFlag("log-level", zapcore.InfoLevel, "Log level")
	flag.Parse()

	log, err := customlog.New(logLevel, logFormat)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	// We need to skip 1 caller level when using zapr
	ctrl.SetLogger(zapr.NewLogger(log.WithOptions(zap.AddCallerSkip(1))))
	defer log.Sync()
	log = log.With(zap.String("interface_name", interfaceName))

	if podCIDR == "" {
		log.Fatal("pod-cidr must be set")
	}
	_, podCidrNet, err := net.ParseCIDR(podCIDR)
	if err != nil {
		log.Fatal("unable to parse pod cidr", zap.Error(err))
	}

	key, err := generateKeys(privateKeyPath, log)
	if err != nil {
		log.Fatal("Unable to get private key", zap.Error(err))
	}

	wgClient, err := wgctrl.New()
	if err != nil {
		log.Fatal("Unable to setup the WireGuard client", zap.Error(err))
	}
	defer wgClient.Close()

	clientConfig := ctrl.GetConfigOrDie()
	client, err := ctrlclient.New(clientConfig, ctrlclient.Options{})
	if err != nil {
		log.Fatal("Unable to create a kubernetes client", zap.Error(err))
	}

	ownNode := &corev1.Node{}
	ctx := context.Background()
	if err := client.Get(ctx, types.NamespacedName{Name: nodeName}, ownNode); err != nil {
		log.Fatal("Unable to load node", zap.Error(err))
	}
	if ownNode.Annotations == nil {
		ownNode.Annotations = map[string]string{}
	}
	ownNode.Annotations[kubernetes.AnnotationKeyPublicKey] = key.PublicKey().String()
	ownNode.Annotations[kubernetes.AnnotationKeyEndpoint] = fmt.Sprintf("%s:%d", kubernetes.GetPrivateNodeAddress(ownNode), listeningPort)
	if err := client.Update(ctx, ownNode); err != nil {
		log.Fatal("Unable to update node", zap.Error(err))
	}

	nodePodCidrIP, nodePodCidrNet, err := net.ParseCIDR(ownNode.Spec.PodCIDR)
	if err != nil {
		log.Fatal("Unable to parse node pod cidr", zap.Error(err))
	}

	wireGuardAddr, err := getWireGuardAddr(nodePodCidrIP)
	if err != nil {
		log.Fatal("Unable to calculate WireGuard IP", zap.Error(err))
	}
	log = log.With(zap.String("wireguard_ip", wireGuardAddr.String()))

	mgr, err := ctrl.NewManager(clientConfig, ctrl.Options{})
	if err != nil {
		log.Fatal("Unable to start manager", zap.Error(err))
	}

	if err := wireguard_interface.Add(
		mgr,
		log,
		interfaceName,
		wgClient,
		*key,
		listeningPort,
		ownNode.Name,
		wireGuardAddr,
	); err != nil {
		log.Fatal("Unable to add the WireGuard interface controller to the controller manager", zap.Error(err))
	}

	if err := cni_config.Add(
		mgr,
		log,
		nodePodCidrNet,
		podCidrNet,
		cniSourceDir,
		cniTargetDir,
	); err != nil {
		log.Fatal("Unable to add the WireGuard interface controller to the controller manager", zap.Error(err))
	}

	if err := route.Add(
		mgr,
		log,
		interfaceName,
		ownNode.Name,
	); err != nil {
		log.Fatal("Unable to add the WireGuard interface controller to the controller manager", zap.Error(err))
	}

	if err := telemetry.Add(mgr, telemetryListenAddress, log); err != nil {
		log.Fatal("Unable to add the telemetry server to the controller manager", zap.Error(err))
	}

	log.Info("Starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		log.Fatal("problem running manager", zap.Error(err))
	}
}

func generateKeys(privateKeyPath string, parentLog *zap.Logger) (*wgtypes.Key, error) {
	log := parentLog.With(zap.String("private_key_path", privateKeyPath))
	_, err := os.Stat(privateKeyPath)
	if os.IsNotExist(err) {
		log.Info("Generating key...")
		privateKey, err := wgtypes.GeneratePrivateKey()
		if err != nil {
			return nil, errors.Wrap(err, "unable to generate key")
		}

		if err := ioutil.WriteFile(privateKeyPath, []byte(privateKey.String()), 0400); err != nil {
			return nil, errors.Wrap(err, "unable to write private key")
		}
		log.Info("Wrote private key to filesystem")

		return &privateKey, nil
	}
	if err != nil {
		return nil, errors.Wrap(err, "unable to check if the private key exists")
	}

	log.Info("Found private key on filesystem")
	b, err := ioutil.ReadFile(privateKeyPath)
	if err != nil {
		return nil, errors.Wrap(err, "unable to read private key")
	}
	privateKey, err := wgtypes.ParseKey(string(b))
	if err != nil {
		return nil, errors.Wrap(err, "unable to read parse private key")
	}
	return &privateKey, nil
}

// Gets the first IP from the given cidr
func getWireGuardAddr(nodePodCidrIP net.IP) (*netlink.Addr, error) {
	wgIP := net.ParseIP(nodePodCidrIP.String())
	wgIP = wgIP.To4()
	return netlink.ParseAddr(fmt.Sprintf("%s/32", wgIP.String()))
}
