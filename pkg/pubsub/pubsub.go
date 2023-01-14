package pubsub

import (
	"fmt"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
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

// TODO: fix this:
/*
ERRO[0035] kicking subscriber for resource of type *v1.WifiNetwork since it timed out accepting the event after 15s, subscriber stack trace:
  goroutine 758 [running]:
  github.com/mgoltzsche/kubemate/pkg/pubsub.(*PubSub).Subscribe(0xc000892500)
  	/work/pkg/pubsub/pubsub.go:42 +0x9c
  github.com/mgoltzsche/kubemate/pkg/storage.(*inMemoryStore).Watch(0xc0008da1e0?, {0x81d7b88?, 0xc0014649c0}, {0xc0012a82cd?, 0x1?})
  	/work/pkg/storage/inmemorystore.go:40 +0xdd
  github.com/mgoltzsche/kubemate/pkg/storage.(*refresher).Watch(0xc00118e0e0, {0x81d7b88?, 0xc0014649c0?}, {0xc0012a82cd?, 0x2a6d6edfed9?})
  	/work/pkg/storage/refresh.go:54 +0x3c
  github.com/mgoltzsche/kubemate/pkg/rest.(*REST).Watch(0x81d7bc0?, {0x81d7b88?, 0xc0014649c0?}, 0x0?)
  	/work/pkg/rest/rest.go:71 +0x37
  k8s.io/apiserver/pkg/endpoints/handlers.ListResource.func1({0x81d56f8, 0xc0005747a0}, 0xc001540d00)
  	/go/pkg/mod/github.com/k3s-io/kubernetes/staging/src/k8s.io/apiserver@v1.26.0-k3s1/pkg/endpoints/handlers/get.go:260 +0x9fb
*/

func (s *PubSub) Subscribe() watch.Interface {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	s.seq++
	buf := make([]byte, 1024)
	i := runtime.Stack(buf, true)
	buf = buf[:i]
	w := &watcher{
		id:     s.seq,
		pubsub: s,
		ch:     make(chan Event, 10),
		stack:  string(buf),
	}
	s.watchers[w.id] = w
	return w
}

func (s *PubSub) Publish(evt Event) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	for _, w := range s.watchers {
		fmt.Printf("## sending event %#v\n", evt)
		sendEvent(evt, w)
		fmt.Printf("## sent event %#v\n", evt)
	}
}

func sendEvent(evt Event, w *watcher) {
	select {
	case w.ch <- evt:
	case <-time.After(20 * time.Second):
		logrus.Errorf("kicking subscriber for resource of type %T since it timed out after 20s accepting the event, subscriber stack trace:\n  %s", evt.Object, strings.ReplaceAll(w.stack, "\n", "\n  "))
		go w.Stop()
	}
}

type watcher struct {
	pubsub *PubSub
	id     int64
	ch     chan Event
	stack  string
}

func (w *watcher) Stop() {
	w.pubsub.mutex.Lock()
	delete(w.pubsub.watchers, w.id)
	ch := w.ch
	w.ch = nil
	w.pubsub.mutex.Unlock()
	if ch != nil {
		close(ch)
		for _ = range ch {
		}
	}
}

func (w *watcher) ResultChan() <-chan Event {
	return w.ch
}
