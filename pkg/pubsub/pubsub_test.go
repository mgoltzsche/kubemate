package pubsub

import (
	"testing"
	"time"

	"github.com/mgoltzsche/kubemate/pkg/resource/fake"
	"github.com/stretchr/testify/require"
)

func TestPubSub(t *testing.T) {
	testee := New()
	w := testee.Subscribe()
	eventCount := 2
	for i := 0; i < eventCount; i++ {
		testee.Publish(Event{
			Type:   Added,
			Object: &fake.FakeResource{},
		})
	}
	go func() {
		time.Sleep(time.Second)
		w.Stop()
		testee.Publish(Event{
			Type:   Added,
			Object: &fake.FakeResource{},
		})
	}()
	count := 0
	for evt := range w.ResultChan() {
		require.Equal(t, Added, evt.Type, "event type")
		require.NotNil(t, evt.Object, "event object")
		_, ok := evt.Object.(*fake.FakeResource)
		require.Truef(t, ok, "event object should be of type FakeResource but is %#v", evt.Object)
		count++
	}
	require.Equal(t, eventCount, count, "received events")
}
