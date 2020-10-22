// Copyright 2019 Copyright (c) 2019 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package framework

import (
	"context"
	"fmt"
	"strings"
	"time"

	gardencorev1beta1 "github.com/gardener/gardener/pkg/apis/core/v1beta1"
	gardencorev1beta1constants "github.com/gardener/gardener/pkg/apis/core/v1beta1/constants"
	"github.com/gardener/gardener/pkg/client/kubernetes"
	"github.com/gardener/gardener/pkg/operation/common"
	"github.com/gardener/gardener/pkg/scheduler/apis/config"
	schedulerconfigv1alpha1 "github.com/gardener/gardener/pkg/scheduler/apis/config/v1alpha1"
	scheduler "github.com/gardener/gardener/pkg/scheduler/controller/shoot"
	kutil "github.com/gardener/gardener/pkg/utils/kubernetes"
	"github.com/gardener/gardener/pkg/utils/kubernetes/health"
	"github.com/gardener/gardener/pkg/utils/retry"

	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// GetSeeds returns all registered seeds
func (f *GardenerFramework) GetSeeds(ctx context.Context) ([]gardencorev1beta1.Seed, error) {
	seeds := &gardencorev1beta1.SeedList{}
	err := f.GardenClient.DirectClient().List(ctx, seeds)
	if err != nil {
		return nil, errors.Wrap(err, "could not get Seeds from Garden cluster")
	}

	return seeds.Items, nil
}

// GetSeed returns the seed and its k8s client
func (f *GardenerFramework) GetSeed(ctx context.Context, seedName string) (*gardencorev1beta1.Seed, kubernetes.Interface, error) {
	seed := &gardencorev1beta1.Seed{}
	err := f.GardenClient.DirectClient().Get(ctx, client.ObjectKey{Name: seedName}, seed)
	if err != nil {
		return nil, nil, errors.Wrap(err, "could not get Seed from Shoot in Garden cluster")
	}

	seedSecretRef := seed.Spec.SecretRef
	seedClient, err := kubernetes.NewClientFromSecret(ctx, f.GardenClient.DirectClient(), seedSecretRef.Namespace, seedSecretRef.Name, kubernetes.WithClientOptions(client.Options{
		Scheme: kubernetes.SeedScheme,
	}))
	if err != nil {
		return nil, nil, errors.Wrap(err, "could not construct Seed client")
	}
	return seed, seedClient, nil
}

// GetShoot gets the test shoot
func (f *GardenerFramework) GetShoot(ctx context.Context, shoot *gardencorev1beta1.Shoot) error {
	return f.GardenClient.DirectClient().Get(ctx, kutil.Key(shoot.Namespace, shoot.Name), shoot)
}

// GetShootProject returns the project of a shoot
func (f *GardenerFramework) GetShootProject(ctx context.Context, shootNamespace string) (*gardencorev1beta1.Project, error) {
	var (
		project = &gardencorev1beta1.Project{}
		ns      = &corev1.Namespace{}
	)
	if err := f.GardenClient.DirectClient().Get(ctx, client.ObjectKey{Name: shootNamespace}, ns); err != nil {
		return nil, errors.Wrap(err, "could not get the Shoot namespace in Garden cluster")
	}

	if ns.Labels == nil {
		return nil, fmt.Errorf("namespace %q does not have any labels", ns.Name)
	}
	projectName, ok := ns.Labels[common.ProjectName]
	if !ok {
		return nil, fmt.Errorf("namespace %q did not contain a project label", ns.Name)
	}

	if err := f.GardenClient.DirectClient().Get(ctx, client.ObjectKey{Name: projectName}, project); err != nil {
		return nil, errors.Wrap(err, "could not get Project in Garden cluster")
	}
	return project, nil
}

// createShootResource creates a shoot from a shoot Object
func (f *GardenerFramework) createShootResource(ctx context.Context, shoot *gardencorev1beta1.Shoot) (*gardencorev1beta1.Shoot, error) {
	err := f.GetShoot(ctx, shoot)
	if err == nil {
		return shoot, apierrors.NewAlreadyExists(gardencorev1beta1.Resource("shoots"), shoot.Namespace+"/"+shoot.Name)
	}
	if !apierrors.IsNotFound(err) {
		return nil, err
	}

	if err := f.GardenClient.DirectClient().Create(ctx, shoot); err != nil {
		return nil, err
	}
	f.Logger.Infof("Shoot resource %s was created!", shoot.Name)
	return shoot, nil
}

// CreateShoot Creates a shoot from a shoot Object and waits until it is successfully reconciled
func (f *GardenerFramework) CreateShoot(ctx context.Context, shoot *gardencorev1beta1.Shoot) error {
	err := retry.UntilTimeout(ctx, 20*time.Second, 5*time.Minute, func(ctx context.Context) (done bool, err error) {
		_, err = f.createShootResource(ctx, shoot)
		if apierrors.IsInvalid(err) || apierrors.IsForbidden(err) || apierrors.IsAlreadyExists(err) {
			return retry.SevereError(err)
		}
		if err != nil {
			f.Logger.Debugf("unable to create shoot %s: %s", shoot.Name, err.Error())
			return retry.MinorError(err)
		}
		return retry.Ok()
	})
	if err != nil {
		return err
	}

	// Then we wait for the shoot to be created
	err = f.WaitForShootToBeCreated(ctx, shoot)
	if err != nil {
		return err
	}

	f.Logger.Infof("Shoot %s was created!", shoot.Name)
	return nil
}

// DeleteShootAndWaitForDeletion deletes the test shoot and waits until it cannot be found any more
func (f *GardenerFramework) DeleteShootAndWaitForDeletion(ctx context.Context, shoot *gardencorev1beta1.Shoot) error {
	err := f.DeleteShoot(ctx, shoot)
	if err != nil {
		return err
	}

	err = f.WaitForShootToBeDeleted(ctx, shoot)
	if err != nil {
		return err
	}

	f.Logger.Infof("Shoot %s was deleted successfully!", shoot.Name)
	return nil
}

// DeleteShoot deletes the test shoot
func (f *GardenerFramework) DeleteShoot(ctx context.Context, shoot *gardencorev1beta1.Shoot) error {
	err := retry.UntilTimeout(ctx, 20*time.Second, 5*time.Minute, func(ctx context.Context) (done bool, err error) {
		err = f.RemoveShootAnnotation(ctx, shoot, common.ShootIgnore)
		if err != nil {
			return retry.MinorError(err)
		}

		// First we annotate the shoot to be deleted.
		err = f.AnnotateShoot(ctx, shoot, map[string]string{
			common.ConfirmationDeletion: "true",
		})
		if err != nil {
			return retry.MinorError(err)
		}

		err = f.GardenClient.DirectClient().Delete(ctx, shoot)
		if err != nil {
			return retry.MinorError(err)
		}

		return retry.Ok()
	})
	if err != nil {
		return err
	}
	return nil
}

// UpdateShoot Updates a shoot from a shoot Object and waits for its reconciliation
func (f *GardenerFramework) UpdateShoot(ctx context.Context, shoot *gardencorev1beta1.Shoot, update func(shoot *gardencorev1beta1.Shoot) error) error {
	err := retry.UntilTimeout(ctx, 20*time.Second, 5*time.Minute, func(ctx context.Context) (done bool, err error) {
		key, err := client.ObjectKeyFromObject(shoot)
		if err != nil {
			return retry.SevereError(err)
		}

		updatedShoot := &gardencorev1beta1.Shoot{}
		if err := f.GardenClient.DirectClient().Get(ctx, key, updatedShoot); err != nil {
			return retry.MinorError(err)
		}

		if err := update(updatedShoot); err != nil {
			return retry.MinorError(err)
		}

		if err := f.GardenClient.DirectClient().Update(ctx, updatedShoot); err != nil {
			f.Logger.Debugf("unable to update shoot %s: %s", updatedShoot.Name, err.Error())
			return retry.MinorError(err)
		}
		*shoot = *updatedShoot
		return retry.Ok()
	})
	if err != nil {
		return err
	}

	// Then we wait for the shoot to be created
	err = f.WaitForShootToBeReconciled(ctx, shoot)
	if err != nil {
		return err
	}

	f.Logger.Infof("Shoot %s was successfully updated!", shoot.Name)
	return nil
}

// HibernateShoot hibernates the test shoot
func (f *GardenerFramework) HibernateShoot(ctx context.Context, shoot *gardencorev1beta1.Shoot) error {
	// return if the shoot is already hibernated
	if shoot.Spec.Hibernation != nil && shoot.Spec.Hibernation.Enabled != nil && *shoot.Spec.Hibernation.Enabled {
		return nil
	}

	err := retry.UntilTimeout(ctx, 20*time.Second, 5*time.Minute, func(ctx context.Context) (done bool, err error) {
		newShoot := shoot.DeepCopy()
		setHibernation(newShoot, true)
		patchedShoot, err := f.MergePatchShoot(ctx, shoot, newShoot)
		if err != nil {
			return retry.MinorError(err)
		}
		*shoot = *patchedShoot

		return retry.Ok()
	})
	if err != nil {
		return err
	}

	err = f.WaitForShootToBeReconciled(ctx, shoot)
	if err != nil {
		return err
	}

	f.Logger.Infof("Shoot %s was hibernated successfully!", shoot.Name)
	return nil
}

// WakeUpShoot wakes up the test shoot from hibernation
func (f *GardenerFramework) WakeUpShoot(ctx context.Context, shoot *gardencorev1beta1.Shoot) error {
	// return if the shoot is already running
	if shoot.Spec.Hibernation == nil || shoot.Spec.Hibernation.Enabled == nil || !*shoot.Spec.Hibernation.Enabled {
		return nil
	}

	err := retry.UntilTimeout(ctx, 20*time.Second, 5*time.Minute, func(ctx context.Context) (done bool, err error) {
		newShoot := shoot.DeepCopy()
		setHibernation(newShoot, false)

		patchedShoot, err := f.MergePatchShoot(ctx, shoot, newShoot)
		if err != nil {
			return retry.MinorError(err)
		}
		*shoot = *patchedShoot
		return retry.Ok()
	})
	if err != nil {
		return err
	}

	err = f.WaitForShootToBeReconciled(ctx, shoot)
	if err != nil {
		return err
	}

	f.Logger.Infof("Shoot %s has been woken up successfully!", shoot.Name)
	return nil
}

// ScheduleShoot set the Spec.Cloud.Seed of a shoot to the specified seed.
// This is the request the Gardener Scheduler executes after a scheduling decision.
func (f *GardenerFramework) ScheduleShoot(ctx context.Context, shoot *gardencorev1beta1.Shoot, seed *gardencorev1beta1.Seed) error {
	shoot.Spec.SeedName = &seed.Name
	return f.GardenClient.DirectClient().Update(ctx, shoot)
}

// WaitForShootToBeCreated waits for the shoot to be created
func (f *GardenerFramework) WaitForShootToBeCreated(ctx context.Context, shoot *gardencorev1beta1.Shoot) error {
	return retry.UntilTimeout(ctx, 30*time.Second, 60*time.Minute, func(ctx context.Context) (done bool, err error) {
		newShoot := &gardencorev1beta1.Shoot{}
		err = f.GardenClient.DirectClient().Get(ctx, client.ObjectKey{Namespace: shoot.Namespace, Name: shoot.Name}, newShoot)
		if err != nil {
			f.Logger.Infof("Error while waiting for shoot to be created: %s", err.Error())
			return retry.MinorError(err)
		}
		*shoot = *newShoot
		completed, msg := ShootCreationCompleted(shoot)
		if completed {
			return retry.Ok()
		}
		f.Logger.Infof("Shoot %s not yet created successfully (%s)", shoot.Name, msg)
		if shoot.Status.LastOperation != nil {
			f.Logger.Infof("%d%%: Shoot State: %s, Description: %s", shoot.Status.LastOperation.Progress, shoot.Status.LastOperation.State, shoot.Status.LastOperation.Description)
		}
		return retry.MinorError(fmt.Errorf("shoot %q was not successfully reconciled", shoot.Name))
	})
}

// WaitForShootToBeDeleted waits for the shoot to be deleted
func (f *GardenerFramework) WaitForShootToBeDeleted(ctx context.Context, shoot *gardencorev1beta1.Shoot) error {
	return retry.UntilTimeout(ctx, 30*time.Second, 60*time.Minute, func(ctx context.Context) (done bool, err error) {
		updatedShoot := &gardencorev1beta1.Shoot{}
		err = f.GardenClient.DirectClient().Get(ctx, client.ObjectKey{Namespace: shoot.Namespace, Name: shoot.Name}, updatedShoot)
		if err != nil {
			if apierrors.IsNotFound(err) {
				return retry.Ok()
			}
			f.Logger.Infof("Error while waiting for shoot to be deleted: %s", err.Error())
			return retry.MinorError(err)
		}
		*shoot = *updatedShoot
		f.Logger.Infof("waiting for shoot %s to be deleted", shoot.Name)
		if shoot.Status.LastOperation != nil {
			f.Logger.Debugf("%d%%: Shoot state: %s, Description: %s", shoot.Status.LastOperation.Progress, shoot.Status.LastOperation.State, shoot.Status.LastOperation.Description)
		}
		return retry.MinorError(fmt.Errorf("shoot %q still exists", shoot.Name))
	})
}

// WaitForShootToBeReconciled waits for the shoot to be successfully reconciled
func (f *GardenerFramework) WaitForShootToBeReconciled(ctx context.Context, shoot *gardencorev1beta1.Shoot) error {
	return retry.UntilTimeout(ctx, 30*time.Second, 60*time.Minute, func(ctx context.Context) (done bool, err error) {
		newShoot := &gardencorev1beta1.Shoot{}
		err = f.GardenClient.DirectClient().Get(ctx, client.ObjectKey{Namespace: shoot.Namespace, Name: shoot.Name}, newShoot)
		if err != nil {
			f.Logger.Infof("Error while waiting for shoot to be reconciled: %s", err.Error())
			return retry.MinorError(err)
		}
		shoot = newShoot
		completed, msg := ShootCreationCompleted(shoot)
		if completed {
			return retry.Ok()
		}
		f.Logger.Infof("Shoot %s not yet reconciled successfully (%s)", shoot.Name, msg)
		if newShoot.Status.LastOperation != nil {
			f.Logger.Debugf("%d%%: Shoot State: %s, Description: %s", shoot.Status.LastOperation.Progress, shoot.Status.LastOperation.State, shoot.Status.LastOperation.Description)
		}
		return retry.MinorError(fmt.Errorf("shoot %q was not successfully reconciled", shoot.Name))
	})
}

// AnnotateShoot adds shoot annotation(s)
func (f *GardenerFramework) AnnotateShoot(ctx context.Context, shoot *gardencorev1beta1.Shoot, annotations map[string]string) error {
	shootCopy := shoot.DeepCopy()

	for annotationKey, annotationValue := range annotations {
		metav1.SetMetaDataAnnotation(&shootCopy.ObjectMeta, annotationKey, annotationValue)
	}

	if _, err := f.MergePatchShoot(ctx, shoot, shootCopy); err != nil {
		return err
	}

	return nil
}

// RemoveShootAnnotation removes an annotation with key <annotationKey> from a shoot object
func (f *GardenerFramework) RemoveShootAnnotation(ctx context.Context, shoot *gardencorev1beta1.Shoot, annotationKey string) error {
	shootCopy := shoot.DeepCopy()
	if len(shootCopy.Annotations) == 0 {
		return nil
	}
	if _, ok := shootCopy.Annotations[annotationKey]; !ok {
		return nil
	}

	// start the update process with Kubernetes
	delete(shootCopy.Annotations, annotationKey)

	if _, err := f.MergePatchShoot(ctx, shoot, shootCopy); err != nil {
		return err
	}
	return nil
}

// WaitForShootToBeUnschedulable waits for the shoot to be unschedulable. This is indicated by Events created by the scheduler on the shoot
func (f *GardenerFramework) WaitForShootToBeUnschedulable(ctx context.Context, shoot *gardencorev1beta1.Shoot) error {
	return retry.Until(ctx, 2*time.Second, func(ctx context.Context) (bool, error) {
		newShoot := &gardencorev1beta1.Shoot{}
		err := f.GardenClient.DirectClient().Get(ctx, client.ObjectKey{Namespace: shoot.Namespace, Name: shoot.Name}, newShoot)
		if err != nil {
			return false, err
		}
		*shoot = *newShoot
		f.Logger.Infof("waiting for shoot %s to be unschedulable", shoot.Name)

		uid := string(shoot.UID)
		kind := "Shoot"
		fieldSelector := f.GardenClient.Kubernetes().CoreV1().Events(shoot.Namespace).GetFieldSelector(&shoot.Name, &shoot.Namespace, &kind, &uid)
		eventList, err := f.GardenClient.Kubernetes().CoreV1().Events(shoot.Namespace).List(ctx, metav1.ListOptions{FieldSelector: fieldSelector.String()})
		if err != nil {
			return false, err
		}
		if shootIsUnschedulable(eventList.Items) {
			return true, nil
		}

		if shoot.Status.LastOperation != nil {
			f.Logger.Debugf("%d%%: Shoot State: %s, Description: %s", shoot.Status.LastOperation.Progress, shoot.Status.LastOperation.State, shoot.Status.LastOperation.Description)
		}
		return false, nil
	})
}

// WaitForShootToBeScheduled waits for the shoot to be scheduled successfully
func (f *GardenerFramework) WaitForShootToBeScheduled(ctx context.Context, shoot *gardencorev1beta1.Shoot) error {
	return retry.Until(ctx, 2*time.Second, func(ctx context.Context) (bool, error) {
		newShoot := &gardencorev1beta1.Shoot{}
		err := f.GardenClient.DirectClient().Get(ctx, client.ObjectKey{Namespace: shoot.Namespace, Name: shoot.Name}, newShoot)
		if err != nil {
			return retry.SevereError(err)
		}
		*shoot = *newShoot
		if shootIsScheduledSuccessfully(&shoot.Spec) {
			return retry.Ok()
		}
		f.Logger.Infof("waiting for shoot %s to be scheduled", shoot.Name)
		if shoot.Status.LastOperation != nil {
			f.Logger.Debugf("%d%%: Shoot State: %s, Description: %s", shoot.Status.LastOperation.Progress, shoot.Status.LastOperation.State, shoot.Status.LastOperation.Description)
		}
		return retry.MinorError(fmt.Errorf("shoot %s is not yet scheduled", shoot.Name))
	})
}

func shootIsScheduledSuccessfully(newSpec *gardencorev1beta1.ShootSpec) bool {
	return newSpec.SeedName != nil
}

func shootIsUnschedulable(events []corev1.Event) bool {
	if len(events) == 0 {
		return false
	}

	for _, event := range events {
		if strings.Contains(event.Message, scheduler.MsgUnschedulable) {
			return true
		}
	}
	return false
}

// MergePatchShoot performs a two way merge patch operation on a shoot object
func (f *GardenerFramework) MergePatchShoot(ctx context.Context, oldShoot, newShoot *gardencorev1beta1.Shoot) (*gardencorev1beta1.Shoot, error) {
	patchBytes, err := kutil.CreateTwoWayMergePatch(oldShoot, newShoot)
	if err != nil {
		return nil, fmt.Errorf("failed to patch bytes")
	}

	patchedShoot, err := f.GardenClient.GardenCore().CoreV1beta1().Shoots(oldShoot.GetNamespace()).Patch(ctx, oldShoot.GetName(), types.StrategicMergePatchType, patchBytes, metav1.PatchOptions{})
	if err == nil {
		*oldShoot = *patchedShoot
	}
	return patchedShoot, err
}

// GetCloudProfile returns the cloudprofile from gardener with the give name
func (f *GardenerFramework) GetCloudProfile(ctx context.Context, name string) (*gardencorev1beta1.CloudProfile, error) {
	cloudProfile := &gardencorev1beta1.CloudProfile{}
	if err := f.GardenClient.DirectClient().Get(ctx, client.ObjectKey{Name: name}, cloudProfile); err != nil {
		return nil, errors.Wrap(err, fmt.Sprintf("could not get CloudProfile '%s' in Garden cluster", name))
	}
	return cloudProfile, nil
}

// DumpState greps all necessary logs and state of the cluster if the test failed
// TODO: dump extension controller namespaces
// TODO: dump logs of gardener extension controllers and other system components
func (f *GardenerFramework) DumpState(ctx context.Context) {
	if f.DisableStateDump {
		return
	}
	if f.GardenClient == nil {
		return
	}

	ctxIdentifier := "[GARDENER]"
	f.Logger.Info(ctxIdentifier)

	if err := f.dumpSeeds(ctx, ctxIdentifier); err != nil {
		f.Logger.Errorf("unable to dump seed status: %s", err.Error())
	}

	// dump events if project namespace set
	if f.ProjectNamespace != "" {
		if err := f.dumpEventsInNamespace(ctx, ctxIdentifier, f.GardenClient, f.ProjectNamespace); err != nil {
			f.Logger.Errorf("unable to dump gardener events from project namespace %s: %s", f.ProjectNamespace, err.Error())
		}
	}
}

// dumpSeeds prints information about all seeds
func (f *GardenerFramework) dumpSeeds(ctx context.Context, ctxIdentifier string) error {
	f.Logger.Infof("%s [SEEDS]", ctxIdentifier)
	seeds := &gardencorev1beta1.SeedList{}
	if err := f.GardenClient.DirectClient().List(ctx, seeds); err != nil {
		return err
	}

	for _, seed := range seeds.Items {
		f.dumpSeed(&seed)
	}
	return nil
}

// dumpSeed prints information about a seed
func (f *GardenerFramework) dumpSeed(seed *gardencorev1beta1.Seed) {
	if err := health.CheckSeed(seed, seed.Status.Gardener); err != nil {
		f.Logger.Printf("Seed %s is %s - Error: %s - Conditions %v", seed.Name, unhealthy, err.Error(), seed.Status.Conditions)
	} else {
		f.Logger.Printf("Seed %s is %s", seed.Name, healthy)
	}
}

func setHibernation(shoot *gardencorev1beta1.Shoot, hibernated bool) {
	if shoot.Spec.Hibernation != nil {
		shoot.Spec.Hibernation.Enabled = &hibernated
	}
	shoot.Spec.Hibernation = &gardencorev1beta1.Hibernation{
		Enabled: &hibernated,
	}
}

// ParseSchedulerConfiguration returns a SchedulerConfiguration from a ConfigMap
func ParseSchedulerConfiguration(configuration *corev1.ConfigMap) (*config.SchedulerConfiguration, error) {
	const configurationFileName = "schedulerconfiguration.yaml"
	if configuration == nil {
		return nil, fmt.Errorf("scheduler Configuration could not be extracted from ConfigMap. The gardener setup with the helm chart creates this config map")
	}

	rawConfig := configuration.Data[configurationFileName]
	byteConfig := []byte(rawConfig)
	scheme := runtime.NewScheme()
	if err := config.AddToScheme(scheme); err != nil {
		return nil, err
	}
	if err := schedulerconfigv1alpha1.AddToScheme(scheme); err != nil {
		return nil, err
	}
	codecs := serializer.NewCodecFactory(scheme)
	configObj, gvk, err := codecs.UniversalDecoder().Decode(byteConfig, nil, nil)
	if err != nil {
		return nil, err
	}
	config, ok := configObj.(*config.SchedulerConfiguration)
	if !ok {
		return nil, fmt.Errorf("got unexpected config type: %v", gvk)
	}
	return config, nil
}

// ScaleGardenerScheduler scales the gardener-scheduler to the desired replicas
func ScaleGardenerScheduler(setupContextTimeout time.Duration, client client.Client, desiredReplicas *int32) (*int32, error) {
	return ScaleDeployment(setupContextTimeout, client, desiredReplicas, "gardener-scheduler", gardencorev1beta1constants.GardenNamespace)
}

// ScaleGardenerControllerManager scales the gardener-controller-manager to the desired replicas
func ScaleGardenerControllerManager(setupContextTimeout time.Duration, client client.Client, desiredReplicas *int32) (*int32, error) {
	return ScaleDeployment(setupContextTimeout, client, desiredReplicas, "gardener-controller-manager", gardencorev1beta1constants.GardenNamespace)
}
