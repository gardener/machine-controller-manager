// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package version

import (
	"k8s.io/klog/v2"
	"runtime"
)

var (
	// These variables typically come from -ldflags settings in build

	// Version shows the etcd-backup-restore binary version.
	Version string
	// GitSHA shows the etcd-backup-restore binary code commit SHA on git.
	GitSHA string
)

// LogVersionInfoWithLevel logs machine-controller-manager version and build information.
func LogVersionInfoWithLevel(debugLevel int32) {
	level := klog.Level(debugLevel)
	klog.V(level).Infof("machine-controller-manager Version: %s\n", Version)
	klog.V(level).Infof("Git SHA: %s\n", GitSHA)
	klog.V(level).Infof("Go Version: %s\n", runtime.Version())
	klog.V(level).Infof("Go OS/Arch: %s/%s\n", runtime.GOOS, runtime.GOARCH)
}
