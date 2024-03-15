//go:build tools
// +build tools

// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

// This package imports things required by build scripts, to force `go mod` to see them as dependencies
package tools

import (
	_ "github.com/onsi/ginkgo/v2/ginkgo"
	_ "golang.org/x/lint/golint"
	_ "k8s.io/code-generator"
)
