/*
Copyright 2024 The Kubernetes Authors.

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

package ingressnginx

import (
	"log/slog"

	"github.com/kubernetes-sigs/ingress2gateway/pkg/i2gw/logging"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func logInfo(message string, callingObjects ...client.Object) {
	slog.Info(message, logging.Provider(string(Name)), logging.ObjectRefs(callingObjects...))
}

func logWarn(message string, callingObjects ...client.Object) {
	slog.Warn(message, logging.Provider(string(Name)), logging.ObjectRefs(callingObjects...))
}

func logError(message string, callingObjects ...client.Object) {
	slog.Error(message, logging.Provider(string(Name)), logging.ObjectRefs(callingObjects...))
}
