// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

// Package drain is used to drain nodes
package drain

import (
	"sync"

	storagev1 "k8s.io/api/storage/v1"
	"k8s.io/klog/v2"
)

// VolumeAttachmentHandler is an handler used to distribute
// incoming VolumeAttachment requests to all listening workers
type VolumeAttachmentHandler struct {
	sync.Mutex
	workers []chan *storagev1.VolumeAttachment
}

// NewVolumeAttachmentHandler returns a new VolumeAttachmentHandler
func NewVolumeAttachmentHandler() *VolumeAttachmentHandler {
	return &VolumeAttachmentHandler{
		Mutex:   sync.Mutex{},
		workers: []chan *storagev1.VolumeAttachment{},
	}
}

func (v *VolumeAttachmentHandler) dispatch(obj interface{}) {
	if len(v.workers) == 0 {
		// As no workers are registered, nothing to do here.
		return
	}

	volumeAttachment := obj.(*storagev1.VolumeAttachment)
	if volumeAttachment == nil {
		klog.Errorf("Couldn't convert to volumeAttachment from object %v", obj)
		return
	}

	klog.V(4).Infof("Dispatching request for PV %s", *volumeAttachment.Spec.Source.PersistentVolumeName)
	defer klog.V(4).Infof("Done dispatching request for PV %s", *volumeAttachment.Spec.Source.PersistentVolumeName)

	v.Lock()
	defer v.Unlock()

	for i, worker := range v.workers {
		klog.V(4).Infof("Dispatching request for PV %s to worker %d/%v", *volumeAttachment.Spec.Source.PersistentVolumeName, i, worker)

		select {
		case worker <- volumeAttachment:
		default:
			klog.Warningf("Worker %d/%v is full. Discarding value.", i, worker)
		}
	}
}

// AddVolumeAttachment is the event handler for VolumeAttachment add
func (v *VolumeAttachmentHandler) AddVolumeAttachment(obj interface{}) {
	klog.V(4).Infof("Adding volume attachment object")
	v.dispatch(obj)
}

// UpdateVolumeAttachment is the event handler for VolumeAttachment update
func (v *VolumeAttachmentHandler) UpdateVolumeAttachment(_, newObj interface{}) {
	klog.V(4).Info("Updating volume attachment object")
	v.dispatch(newObj)
}

// AddWorker is the method used to add a new worker
func (v *VolumeAttachmentHandler) AddWorker() chan *storagev1.VolumeAttachment {
	// chanSize is the channel buffer size to hold requests.
	// This assumes that not more than 20 unprocessed objects would exist at a given time.
	// On bufferring requests beyond this the channel will start dropping writes
	const chanSize = 20

	klog.V(4).Infof("Adding new worker. Current active workers %d - %v", len(v.workers), v.workers)

	v.Lock()
	defer v.Unlock()

	newWorker := make(chan *storagev1.VolumeAttachment, chanSize)
	v.workers = append(v.workers, newWorker)

	klog.V(4).Infof("Successfully added new worker %v. Current active workers %d - %v", newWorker, len(v.workers), v.workers)
	return newWorker
}

// DeleteWorker is the method used to delete an existing worker
func (v *VolumeAttachmentHandler) DeleteWorker(desiredWorker chan *storagev1.VolumeAttachment) {
	klog.V(4).Infof("Deleting an existing worker %v. Current active workers %d - %v", desiredWorker, len(v.workers), v.workers)

	v.Lock()
	defer v.Unlock()

	finalWorkers := []chan *storagev1.VolumeAttachment{}

	for i, worker := range v.workers {
		if worker == desiredWorker {
			close(worker)
			klog.V(4).Infof("Deleting worker %d from worker list", i)
		} else {
			finalWorkers = append(finalWorkers, worker)
		}
	}

	v.workers = finalWorkers
	klog.V(4).Infof("Successfully removed worker. Current active workers %d - %v", len(v.workers), v.workers)
}
