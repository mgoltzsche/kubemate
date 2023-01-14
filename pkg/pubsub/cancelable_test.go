package pubsub

import (
	"context"
	"testing"
	"time"

	"github.com/mgoltzsche/kubemate/pkg/resource/fake"
	"github.com/stretchr/testify/require"
)

func TestCancelable(t *testing.T) {
	eventCount := 2
	w := &fakeWatcher{
		ch: make(chan Event, eventCount),
	}
	for i := 0; i < eventCount; i++ {
		w.ch <- Event{
			Type:   Added,
			Object: &fake.FakeResource{},
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	testee := Cancelable(ctx, w)
	go func() {
		time.Sleep(time.Second)
		cancel()
	}()
	count := 0
	for evt := range testee.ResultChan() {
		require.Equal(t, Added, evt.Type, "event type")
		require.NotNil(t, evt.Object, "event object")
		_, ok := evt.Object.(*fake.FakeResource)
		require.Truef(t, ok, "event object should be of type FakeResource but is %#v", evt.Object)
		count++
	}
	require.Equal(t, eventCount, count, "received events")
}

type fakeWatcher struct {
	ch chan Event
}

func (w *fakeWatcher) Stop() {
	close(w.ch)
	for _ = range w.ch {
	}
}

func (w *fakeWatcher) ResultChan() <-chan Event {
	return w.ch
}
