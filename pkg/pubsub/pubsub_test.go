package pubsub

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestPubSub(t *testing.T) {
	testee := New()
	w := testee.Subscribe(context.Background(), Selector{
		Type:      &corev1.Secret{},
		Namespace: "mynamespace",
		Name:      "myresource",
	})
	eventCount := 3
	go func() {
		for i := 0; i < eventCount; i++ {
			testee.Publish(Event{
				Type: Modified,
				Object: &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "myresource",
						Namespace: "mynamespace",
					},
				},
			})
		}
		testee.Publish(Event{ // ignore different name
			Type: Modified,
			Object: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "myresource-with-different-name",
					Namespace: "mynamespace",
				},
			},
		})
		testee.Publish(Event{ // ignore different namespace
			Type: Modified,
			Object: &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "myresource",
					Namespace: "different-namespace",
				},
			},
		})
		testee.Publish(Event{ // ignore different type
			Type: Modified,
			Object: &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "myresource",
					Namespace: "mynamespace",
				},
			},
		})
	}()
	time.Sleep(100 * time.Millisecond)
	go func() {
		time.Sleep(time.Second)
		w.Stop()
		testee.Publish(Event{
			Type:   Added,
			Object: &corev1.Secret{},
		})
	}()
	count := 0
	for evt := range w.ResultChan() {
		require.Equal(t, Modified, evt.Type, "event type")
		require.NotNil(t, evt.Object, "event object")
		_, ok := evt.Object.(*corev1.Secret)
		require.Truef(t, ok, "event object should be of type Secret but is %#v", evt.Object)
		count++
	}
	require.Equal(t, eventCount, count, "received events")
}
