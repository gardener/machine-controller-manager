/*
Copyright 2015 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.

This file was copied and modified from the kubernetes/kubernetes project
https://github.com/kubernetes/kubernetes/blob/release-1.8/pkg/util/configz/configz.go
*/

package configz

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
)

var (
	configsGuard sync.RWMutex
	configs      = map[string]*Config{}
)

// Config is the utility used to configure
type Config struct {
	val interface{}
}

// InstallHandler used to handle /configz endpoint
func InstallHandler(m mux) {
	m.Handle("/configz", http.HandlerFunc(handle))
}

type mux interface {
	Handle(string, http.Handler)
}

// New creates a new config
func New(name string) (*Config, error) {
	configsGuard.Lock()
	defer configsGuard.Unlock()
	if _, found := configs[name]; found {
		return nil, fmt.Errorf("register config %q twice", name)
	}
	newConfig := Config{}
	configs[name] = &newConfig
	return &newConfig, nil
}

// Delete delete's a config
func Delete(name string) {
	configsGuard.Lock()
	defer configsGuard.Unlock()
	delete(configs, name)
}

// Set set's up the config
func (v *Config) Set(val interface{}) {
	configsGuard.Lock()
	defer configsGuard.Unlock()
	v.val = val
}

// MarshalJSON marshal the JSON
func (v *Config) MarshalJSON() ([]byte, error) {
	return json.Marshal(v.val)
}

func handle(w http.ResponseWriter, r *http.Request) {
	if err := write(w); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func write(w io.Writer) error {
	var b []byte
	var err error
	func() {
		configsGuard.RLock()
		defer configsGuard.RUnlock()
		b, err = json.Marshal(configs)
	}()
	if err != nil {
		return fmt.Errorf("error marshaling json: %v", err)
	}
	_, err = w.Write(b)
	return err
}
