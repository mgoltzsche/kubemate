package pubsub

import (
	"context"
	"reflect"
	goruntime "runtime"
	"strings"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
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
	stack    string
}

func New() *PubSub {
	return &PubSub{watchers: map[int64]*watcher{}}
}

type Selector struct {
	Type      runtime.Object
	Namespace string
	Name      string
	t         reflect.Type
}

func (s *Selector) reflectType() reflect.Type {
	if s.t == nil {
		s.t = reflect.TypeOf(s.Type)
	}
	return s.t
}

func (s *PubSub) Subscribe(ctx context.Context, filter Selector) watch.Interface {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	s.seq++
	buf := make([]byte, 1024)
	i := goruntime.Stack(buf, true)
	buf = buf[:i]
	ctx, cancel := context.WithCancel(ctx)
	w := &watcher{
		id:     s.seq,
		cancel: cancel,
		pubsub: s,
		ch:     make(chan Event, 10),
		stack:  string(buf),
		filter: filter,
	}
	s.watchers[w.id] = w
	go func() {
		<-ctx.Done()
		w.Stop()
	}()
	return w
}

func (s *PubSub) Publish(evt Event) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	m, err := meta.Accessor(evt.Object)
	if err != nil {
		return
	}
	objType := reflect.TypeOf(evt.Object)
	for _, w := range s.watchers {
		if isSubscribed(w.filter, evt, m, objType) {
			sendEvent(evt, w)
		}
	}
}

func isSubscribed(s Selector, evt watch.Event, m metav1.Object, objType reflect.Type) bool {
	if evt.Type == watch.Error {
		return true
	}
	if s.Type != nil && s.reflectType() != objType {
		return false
	}
	if s.Name != "" && s.Name != m.GetName() {
		return false
	}
	if s.Namespace != "" && s.Namespace != m.GetNamespace() {
		return false
	}
	return true
}

func sendEvent(evt Event, w *watcher) {
	select {
	case w.ch <- evt:
	case <-time.After(20 * time.Second):
		logrus.Errorf("kicking %T event subscriber since it timed out accepting the event after 20s, subscriber stack trace:\n  %s", evt.Object, strings.ReplaceAll(w.stack, "\n", "\n  "))
		go w.Stop()
	}
}

type watcher struct {
	pubsub *PubSub
	id     int64
	cancel context.CancelFunc
	ch     chan Event
	stack  string
	filter Selector
}

func (w *watcher) Stop() {
	w.pubsub.mutex.Lock()
	delete(w.pubsub.watchers, w.id)
	ch := w.ch
	w.ch = nil
	w.pubsub.mutex.Unlock()
	if ch != nil {
		close(ch)
		w.cancel()
		for _ = range ch {
		}
	}
}

func (w *watcher) ResultChan() <-chan Event {
	return w.ch
}
