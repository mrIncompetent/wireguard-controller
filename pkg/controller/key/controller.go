package key

import (
	"fmt"
	"io/ioutil"
	"os"
	"time"

	"github.com/prometheus/client_golang/prometheus/promauto"
	"go.uber.org/zap"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
	"k8s.io/apimachinery/pkg/util/rand"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"

	"github.com/mrincompetent/wireguard-controller/pkg/source"
)

const (
	name = "key_controller"
)

type keyStore interface {
	Set(key wgtypes.Key)
	Get() wgtypes.Key
}

type Reconciler struct {
	client.Client
	log                *zap.Logger
	privateKeyFilePath string
	keyStore           keyStore
}

func Add(
	mgr ctrl.Manager,
	log *zap.Logger,
	privateKeyFilePath string,
	keyStore keyStore,
	metricFactory promauto.Factory,
) error {
	options := controller.Options{
		MaxConcurrentReconciles: 1,
		Reconciler: &Reconciler{
			Client: mgr.GetClient(),
			log: log.Named(name).With(
				zap.String("private_key_file", privateKeyFilePath),
			),
			privateKeyFilePath: privateKeyFilePath,
			keyStore:           keyStore,
		},
	}

	c, err := controller.New(name, mgr, options)
	if err != nil {
		return err
	}

	return c.Watch(source.NewIntervalSource(5*time.Second), &handler.EnqueueRequestForObject{})
}

func (r *Reconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	log := r.log.With(zap.String("sync_id", rand.String(12)))
	log.Debug("Processing")

	content, err := ioutil.ReadFile(r.privateKeyFilePath)
	if err != nil {
		if !os.IsNotExist(err) {
			return ctrl.Result{}, fmt.Errorf("failed to load key from file '%s': %w", r.privateKeyFilePath, err)
		}

		log.Debug("Generating new private key")

		key, err := wgtypes.GeneratePrivateKey()
		if err != nil {
			return ctrl.Result{}, fmt.Errorf("unable to generate key: %w", err)
		}

		if err := ioutil.WriteFile(r.privateKeyFilePath, []byte(key.String()), 0400); err != nil {
			return ctrl.Result{}, fmt.Errorf("unable to write private key to '%s': %w", r.privateKeyFilePath, err)
		}

		r.keyStore.Set(key)

		log.Info("Generated a new private key")

		return ctrl.Result{}, nil
	}

	currentKey, err := wgtypes.ParseKey(string(content))
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("unable to read parse private key: %w", err)
	}

	existingKey := r.keyStore.Get()
	if existingKey.String() != currentKey.String() {
		r.keyStore.Set(currentKey)
	}

	return ctrl.Result{}, nil
}
