// SPDX-FileCopyrightText: 2026 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package cluster

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	_ "embed"

	"github.com/gardener/machine-controller-manager/pkg/apis/machine/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	e2efwkenv "sigs.k8s.io/e2e-framework/pkg/env"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
	"sigs.k8s.io/e2e-framework/pkg/envfuncs"
	"sigs.k8s.io/e2e-framework/support/kwok"
)

//go:embed kwok-config.yaml
var kwokctlConfig []byte

type Env struct {
	Name      string
	Namespace string
	Ctx       context.Context
	Cfg       *envconf.Config
}

func New(name, namespace string) Env {
	return Env{
		Name:      name,
		Namespace: namespace,
		Ctx:       context.Background(),
		Cfg:       e2efwkenv.New().EnvConf(),
	}
}

func (env *Env) SetupCluster() (err error) {
	if err = env.createCluster(); err != nil {
		return
	}

	if err = env.deployCRDs(); err != nil {
		return
	}

	// Register MCM API objects with the cluster scheme
	scheme := env.Cfg.Client().Resources().GetScheme()
	if err = v1alpha1.AddToScheme(scheme); err != nil {
		return
	}

	// For every MachineClass that's added, create fake secrets that
	// are part of its SecretRef and CredentialsSecretRef.
	// This is needed for the testing framework as well as for MCM
	// machine reconciliation, wherein it passes the secret when
	// issuing a CreateMachine() call. Ref triggerCreationFlow()
	return env.watchMCCForSecretRefs()
}

func (env *Env) DeleteCluster() (err error) {
	destroyClusterFunc := envfuncs.DestroyCluster(env.Name)
	_, err = destroyClusterFunc(env.Ctx, env.Cfg)
	return
}

func (env *Env) createCluster() (err error) {
	// Using the direct path doesn't work since it looks for the file
	// relative to the caller, so the config file is embedded and a
	// temporary file path is passed in order to create the cluster.
	configPath := filepath.Join(os.TempDir(), "kwok-config.yaml")
	if err = os.WriteFile(configPath, kwokctlConfig, 0644); err != nil {
		return
	}
	defer os.Remove(configPath)

	createClusterFunc := envfuncs.CreateClusterWithConfig(
		kwok.NewProvider(), env.Name, configPath,
	)
	env.Ctx, err = createClusterFunc(env.Ctx, env.Cfg)
	if err != nil {
		return
	}
	createNamespaceFunc := envfuncs.CreateNamespace(env.Namespace)
	env.Ctx, err = createNamespaceFunc(env.Ctx, env.Cfg)
	if err != nil && !apierrors.IsAlreadyExists(err) {
		return
	}
	return nil
}

func (env *Env) deployCRDs() (err error) {
	// TODO: think if there's a better caller-agnostic way to get this path
	_, f, _, ok := runtime.Caller(0)
	if !ok {
		err = fmt.Errorf("could not find the path to crds")
		return
	}
	mcmRepoPath := filepath.Join(filepath.Dir(f), "..", "..", "..")
	crdsDir := filepath.Join(mcmRepoPath, "kubernetes", "crds")
	if _, err = os.Stat(crdsDir); err != nil {
		return fmt.Errorf("crdsDir %q not found: %v", crdsDir, err)
	}

	installCRDs := envfuncs.SetupCRDs(crdsDir, "*")
	_, err = installCRDs(env.Ctx, env.Cfg)
	return
}

func (env *Env) watchMCCForSecretRefs() error {
	return env.Cfg.Client().Resources().
		Watch(&v1alpha1.MachineClassList{}).
		WithAddFunc(func(obj any) {
			mcc, ok := obj.(*v1alpha1.MachineClass)
			if !ok || mcc.CredentialsSecretRef == nil || mcc.SecretRef == nil {
				return
			}
			secretMap := map[string]string{
				mcc.CredentialsSecretRef.Name: mcc.CredentialsSecretRef.Namespace,
				mcc.SecretRef.Name:            mcc.SecretRef.Namespace,
			}

			for name, ns := range secretMap {
				err := env.createFakeSecret(name, ns)
				if err != nil && !apierrors.IsAlreadyExists(err) {
					fmt.Printf("ERR: Creating secret %q for %q: %v\n",
						name, mcc.Name, err,
					)
					return
				}
			}
		}).
		Start(env.Ctx)
}

func (env *Env) createFakeSecret(name, namespace string) (err error) {
	secret := corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Data: map[string][]byte{},
		StringData: map[string]string{
			"userData": "fake-data",
		},
		Type: corev1.SecretTypeOpaque,
	}
	err = env.Cfg.Client().Resources().Create(env.Ctx, &secret)
	if err != nil && !apierrors.IsAlreadyExists(err) {
		return fmt.Errorf("error creating secret %s: %w", name, err)
	}
	return
}
