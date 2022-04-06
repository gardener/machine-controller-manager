/*
Copyright (c) 2020 SAP SE or an SAP affiliate company. All rights reserved.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Package permits is used to provide permitGiver which maintains a sync map whose values can be
// deleted if not accessed for a configured time
package permits

import (
	"sync"
	"time"

	"k8s.io/klog/v2"
)

// PermitGiver provides different operations regarding permit for a given key
type PermitGiver interface {
	RegisterPermits(key string, numPermits int)
	TryPermit(key string, timeout time.Duration) bool
	ReleasePermit(key string)
	DeletePermits(key string)
	Close()
}

type permit struct {
	lastAcquiredPermitTime time.Time
	c                      chan struct{}
}

// NewPermitGiver returns a new PermitGiver
func NewPermitGiver(stalePermitKeyTimeout time.Duration, janitorFrequency time.Duration) PermitGiver {
	stopC := make(chan struct{})
	pg := permitGiver{
		keyPermitsMap: sync.Map{},
		stopC:         stopC,
	}
	go func() {
		ticker := time.NewTicker(janitorFrequency)
		klog.Info("Janitor initialized")
		for {
			select {
			case <-stopC:
				return
			case <-ticker.C:
				pg.cleanupStalePermitEntries(stalePermitKeyTimeout)
			}
		}
	}()
	return &pg
}

type permitGiver struct {
	keyPermitsMap sync.Map
	stopC         chan struct{}
}

func (pg *permitGiver) RegisterPermits(key string, numPermits int) {
	if pg.isClosed() {
		return
	}
	p := permit{
		c:                      make(chan struct{}, numPermits),
		lastAcquiredPermitTime: time.Now(),
	}
	_, loaded := pg.keyPermitsMap.LoadOrStore(key, p)
	if loaded {
		close(p.c)
		klog.V(4).Infof("Permits have already registered for key: %s", key)
	} else {
		klog.V(2).Infof("Permit registered for key: %s", key)
	}
}

func (pg *permitGiver) DeletePermits(key string) {
	if pg.isClosed() {
		return
	}
	if obj, ok := pg.keyPermitsMap.Load(key); ok {
		p := obj.(permit)
		close(p.c)
		pg.keyPermitsMap.Delete(key)
	}
}

func (pg *permitGiver) TryPermit(key string, timeout time.Duration) bool {
	if pg.isClosed() {
		return false
	}
	obj, ok := pg.keyPermitsMap.Load(key)
	if !ok {
		klog.Errorf("There is no permit registered for key: %s", key)
		return false
	}
	p := obj.(permit)
	tick := time.NewTicker(timeout)
	defer tick.Stop()
	for {
		select {
		case p.c <- struct{}{}:
			p.lastAcquiredPermitTime = time.Now()
			return true
		case <-tick.C:
			return false
		case <-pg.stopC:
			klog.V(2).Infof("PermitGiver has been stopped")
			return false
		}
	}
}

func (pg *permitGiver) ReleasePermit(key string) {
	obj, ok := pg.keyPermitsMap.Load(key)
	if !ok {
		klog.Errorf("There is no permit registered for key: %s", key)
		return
	}
	p := obj.(permit)
	if len(p.c) == 0 {
		klog.V(4).Infof("There are currently no permits allocated for key: %s. Nothing to release", key)
		return
	}
	select {
	case <-p.c:
	default:
	}
}

func (pg *permitGiver) isClosed() bool {
	select {
	case <-pg.stopC:
		klog.Errorf("PermitGiver has been closed, no operations are now permitted.")
		return true
	default:
		return false
	}
}

func (pg *permitGiver) isPermitAllocated(key string) bool {
	if pg.isClosed() {
		return false
	}
	_, ok := pg.keyPermitsMap.Load(key)
	return ok
}

func (pg *permitGiver) isPermitAcquired(key string) bool {
	if obj, ok := pg.keyPermitsMap.Load(key); ok {
		p := obj.(permit)
		if len(p.c) != 0 {
			return true
		}
	} else {
		klog.V(4).Infof("couldn't find a permit corresponding to key: %s", key)
	}
	return false
}

func (pg *permitGiver) cleanupStalePermitEntries(stalePermitKeyTimeout time.Duration) {
	pg.keyPermitsMap.Range(func(key, value interface{}) bool {
		p := value.(permit)
		timeout := time.Now().Add(-stalePermitKeyTimeout).Sub(p.lastAcquiredPermitTime)
		if timeout > 0 && len(p.c) == 0 {
			pg.keyPermitsMap.Delete(key)
		}
		return true
	})
}

func (pg *permitGiver) Close() {
	close(pg.stopC)
	// close all permit channels
	pg.keyPermitsMap.Range(func(key, value interface{}) bool {
		p := value.(permit)
		close(p.c)
		return true
	})
}
