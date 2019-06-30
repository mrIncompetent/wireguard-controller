package kubernetes

import (
	"time"

	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

type IntervalSource struct {
	interval time.Duration
}

var (
	staticRequest = reconcile.Request{NamespacedName: types.NamespacedName{
		Name:      "statuc",
		Namespace: "default",
	}}
)

func NewTickerSource(interval time.Duration) *IntervalSource {
	return &IntervalSource{
		interval: interval,
	}
}

func (i *IntervalSource) Start(handler handler.EventHandler, queue workqueue.RateLimitingInterface, _ ...predicate.Predicate) error {
	ticker := time.NewTicker(i.interval)
	// Ensure we always add an initial event
	queue.Add(staticRequest)
	go func() {
		for range ticker.C {
			if queue.ShuttingDown() {
				ticker.Stop()
				return
			}
			queue.Add(staticRequest)
		}
	}()
	return nil
}
