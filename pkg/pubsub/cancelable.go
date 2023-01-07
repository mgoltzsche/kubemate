package pubsub

import (
	"context"

	"k8s.io/apimachinery/pkg/watch"
)

type cancelableWatcher struct {
	delegate watch.Interface
	ch       chan watch.Event
	ctx      context.Context
}

func Cancelable(ctx context.Context, delegate watch.Interface) watch.Interface {
	return &cancelableWatcher{
		delegate: delegate,
		ctx:      ctx,
	}
}

func (w *cancelableWatcher) ResultChan() <-chan watch.Event {
	if w.ch != nil {
		panic("ResultChan() cannot be called more than once")
	}
	w.ch = make(chan watch.Event)
	ch := w.delegate.ResultChan()
	done := w.ctx.Done()
	go func() {
		defer close(w.ch)
		for {
			select {
			case evt, ok := <-ch:
				if !ok {
					return
				}
				w.ch <- evt
			case <-done:
				w.Stop()
				continue
			}
		}
	}()
	return w.ch
}

func (w *cancelableWatcher) Stop() {
	w.delegate.Stop()
	for _ = range w.ch {
	}
}
