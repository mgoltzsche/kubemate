package pubsub

import (
	"context"
	"sync"

	"k8s.io/apimachinery/pkg/watch"
)

type Event = watch.Event

type EventType = watch.EventType

type Interface = watch.Interface

const (
	Added    = watch.Added
	Modified = watch.Modified
	Deleted  = watch.Deleted
)

type PubSub struct {
	mutex    sync.RWMutex
	watchers map[int64]*watcher
	seq      int64
}

func New() *PubSub {
	return &PubSub{watchers: map[int64]*watcher{}}
}

func (s *PubSub) Subscribe(ctx context.Context) watch.Interface {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	s.seq++
	w := &watcher{
		id:     s.seq,
		pubsub: s,
		ch:     make(chan Event, 10),
	}
	s.watchers[w.id] = w
	return w
}

func (s *PubSub) Publish(evt Event) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	for _, w := range s.watchers {
		w.ch <- evt
	}
}

type watcher struct {
	pubsub *PubSub
	id     int64
	ch     chan Event
}

func (w *watcher) Stop() {
	w.pubsub.mutex.Lock()
	delete(w.pubsub.watchers, w.id)
	w.pubsub.mutex.Unlock()
	close(w.ch)
}

func (w *watcher) ResultChan() <-chan Event {
	return w.ch
}
