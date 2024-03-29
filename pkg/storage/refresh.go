package storage

import (
	"context"
	"sync"
	"time"

	"github.com/mgoltzsche/kubemate/pkg/resource"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
)

// refresher is a store that is frequently refreshed as long as somebody is using it.
type refresher struct {
	Interface
	interval time.Duration
	refresh  func()
}

func RefreshPeriodically(store Interface, interval time.Duration, fn func(Interface)) Interface {
	return &refresher{
		Interface: store,
		interval:  interval,
		refresh: rateLimit(interval, func() {
			fn(store)
		}),
	}
}

func rateLimit(interval time.Duration, fn func()) func() {
	mutex := &sync.Mutex{}
	refreshing := false
	var lastRefresh time.Time
	return func() {
		mutex.Lock()
		if refreshing || time.Now().Before(lastRefresh.Add(interval)) {
			mutex.Unlock()
			return
		}
		lastRefresh = time.Now()
		refreshing = true
		mutex.Unlock()
		defer func() {
			mutex.Lock()
			defer mutex.Unlock()
			refreshing = false
		}()
		fn()
	}
}

func (r *refresher) Watch(ctx context.Context, resourceVersion string) (watch.Interface, error) {
	w, err := r.Interface.Watch(ctx, resourceVersion)
	if err != nil {
		return nil, err
	}
	return refreshingWatcher(w, r.interval, r.refresh), nil
}

func (r *refresher) List(l runtime.Object) error {
	go r.refresh()
	return r.Interface.List(l)
}

func (r *refresher) Get(key string, res resource.Resource) error {
	err := r.Interface.Get(key, res)
	if errors.IsNotFound(err) {
		r.refresh()
		return r.Interface.Get(key, res)
	}
	return err
}

type watcher struct {
	delegate watch.Interface
	interval time.Duration
	refresh  func()
	ch       chan watch.Event
}

func refreshingWatcher(delegate watch.Interface, interval time.Duration, refresh func()) *watcher {
	return &watcher{
		delegate: delegate,
		interval: interval,
		refresh:  refresh,
	}
}

func (w *watcher) ResultChan() <-chan watch.Event {
	if w.ch != nil {
		panic("ResultChan() cannot be called more than once")
	}
	w.ch = make(chan watch.Event)
	ch := w.delegate.ResultChan()
	go func() {
		defer close(w.ch)
		time.Sleep(time.Second)
		for {
			select {
			case evt, ok := <-ch:
				if !ok {
					return
				}
				w.ch <- evt
			case <-time.After(w.interval):
				w.refresh()
				continue
			}
		}
	}()
	return w.ch
}

func (w *watcher) Stop() {
	w.delegate.Stop()
	for _ = range w.ch {
	}
}
