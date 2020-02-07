package common

import (
	"context"
	"fmt"
	"sync"
	"time"
	"webhooks/common/data"
	"webhooks/common/storage"
)

// collects data and groups it in batches based on the maxBufferSize and flushTimeout config options
func NewObjectBuffer(store storage.Store, maxBufferSize int, flushTimeout time.Duration) *ObjectBuffer {
	return &ObjectBuffer{
		flushChan:     make(chan struct{}, 1),
		closeChan:     make(chan struct{}, 1),
		inChan:        make(chan *data.WebHookObject),
		storage:       store,
		flushTimeout:  flushTimeout,
		maxBufferSize: maxBufferSize,
	}
}

type ObjectBuffer struct {
	flushChan     chan struct{}
	closeChan     chan struct{}
	runOnce       sync.Once
	inChan        chan *data.WebHookObject
	storage       storage.Store
	flushTimeout  time.Duration
	maxBufferSize int
}

func (b *ObjectBuffer) run() *ObjectBuffer {
	b.runOnce.Do(func() {
		go func() {

			defer func() {
				close(b.closeChan)
				close(b.flushChan)
				close(b.inChan)
			}()

			flushTicker := time.NewTicker(b.flushTimeout)

			pending := make([]*data.WebHookObject, 0)

			for {
				select {
				case _ = <-b.flushChan:
					flushTicker.Stop()
					//TODO lots of this can be improved here
					//TODO needs some proper error handling
					err := b.storage.Put(context.TODO(), pending)
					if err != nil {
						fmt.Printf(err.Error())
					} else {

					}
					//TODO this should be done only when there's not an error
					pending = make([]*data.WebHookObject, 0)

					flushTicker = time.NewTicker(b.flushTimeout)
				case v := <-b.inChan:
					pending = append(pending, v)
					if len(pending) >= b.maxBufferSize {
						b.Flush()
					}

				case <-flushTicker.C:
					b.Flush()
				}
			}
		}()
	})

	return b
}

func (b *ObjectBuffer) Add(item *data.WebHookObject) bool {
	select {
	case b.run().inChan <- item:
		return true
	default:
		return false
	}
}

func (b *ObjectBuffer) Flush() bool {
	return b.signal(b.flushChan)
}

func (b *ObjectBuffer) Close() bool {
	return b.signal(b.closeChan)
}

func (b *ObjectBuffer) signal(ch chan struct{}) bool {
	select {
	case ch <- struct{}{}:
		return true
	default:
		return false
	}
}
