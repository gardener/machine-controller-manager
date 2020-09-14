package backoff_manager

import (
	"time"

	"k8s.io/client-go/util/flowcontrol"
)

type KeyFunc func(interface{}) string

type BackoffManager struct {
	safeBackoffer *flowcontrol.Backoff
	keyFunc       KeyFunc
}

func NewBackoffManager(keyFunc KeyFunc, initialDelay, maxDelay time.Duration) *BackoffManager {
	return &BackoffManager{
		safeBackoffer: flowcontrol.NewBackOff(initialDelay, maxDelay),
		keyFunc:       keyFunc,
	}
}

func (b *BackoffManager) When(item interface{}) time.Duration {
	key := b.keyFunc(item)
	if len(key) == 0 {
		return 0
	}

	b.safeBackoffer.Next(key, time.Now())
	return b.safeBackoffer.Get(key)
}

func (b *BackoffManager) Forget(item interface{}) {
	key := b.keyFunc(item)
	if len(key) == 0 {
		return
	}

	b.safeBackoffer.DeleteEntry(key)
}

func (b *BackoffManager) NumRequeues(_ interface{}) int {
	return 0
}
