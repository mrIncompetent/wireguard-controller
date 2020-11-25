package source

import (
	"errors"
	"time"

	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var (
	staticRequest = reconcile.Request{NamespacedName: types.NamespacedName{
		Name:      "static",
		Namespace: "default",
	}}

	ErrStartCalledBeforeDependencyInjection = errors.New("must call InjectStop on IntervalSource before calling Start")
)

type IntervalSource struct {
	interval time.Duration
	stop     <-chan struct{}
}

func NewIntervalSource(interval time.Duration) *IntervalSource {
	return &IntervalSource{
		interval: interval,
	}
}

func (i *IntervalSource) Start(h handler.EventHandler, queue workqueue.RateLimitingInterface, _ ...predicate.Predicate) error {
	if i.stop == nil {
		return ErrStartCalledBeforeDependencyInjection
	}

	ticker := time.NewTicker(i.interval)
	// Ensure we always add an initial event
	queue.Add(staticRequest)

	go func() {
		for {
			select {
			case <-ticker.C:
				queue.Add(staticRequest)
			case <-i.stop:
				ticker.Stop()

				return
			}
		}
	}()

	return nil
}

func (i *IntervalSource) InjectStopChannel(stop <-chan struct{}) error {
	if i.stop == nil {
		i.stop = stop
	}

	return nil
}
