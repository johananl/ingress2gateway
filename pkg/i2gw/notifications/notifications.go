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

package notifications

import (
	"fmt"
	"strings"
	"sync"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/olekukonko/tablewriter"
)

const (
	InfoNotification    MessageType = "INFO"
	WarningNotification MessageType = "WARNING"
	ErrorNotification   MessageType = "ERROR"
)

type MessageType string

type Notification struct {
	Type           MessageType
	Message        string
	CallingObjects []client.Object
}

type NotificationAggregator struct {
	mutex         sync.Mutex
	Notifications map[string][]Notification
}

// NewNotificationAggregator creates a new NotificationAggregator instance.
func NewNotificationAggregator() *NotificationAggregator {
	return &NotificationAggregator{
		Notifications: make(map[string][]Notification),
	}
}

// Notifier provides a convenient way to dispatch notifications for a specific provider.
// It should be stored in structs rather than used as a package-level variable.
type Notifier struct {
	aggregator   *NotificationAggregator
	providerName string
}

// NewNotifier creates a Notifier bound to a specific provider name.
func NewNotifier(aggregator *NotificationAggregator, providerName string) *Notifier {
	return &Notifier{
		aggregator:   aggregator,
		providerName: providerName,
	}
}

// Notify dispatches a notification to the aggregator.
func (n *Notifier) Notify(mType MessageType, message string, callingObject ...client.Object) {
	if n == nil || n.aggregator == nil {
		return
	}
	notification := NewNotification(mType, message, callingObject...)
	n.aggregator.DispatchNotification(notification, n.providerName)
}

// DispatchNotification is used to send a notification to the NotificationAggregator
func (na *NotificationAggregator) DispatchNotification(notification Notification, ProviderName string) {
	na.mutex.Lock()
	na.Notifications[ProviderName] = append(na.Notifications[ProviderName], notification)
	na.mutex.Unlock()
}

// CreateNotificationTables takes all generated notifications and returns a map[string]string
// that displays the notifications in a tabular format based on provider
func (na *NotificationAggregator) CreateNotificationTables() map[string]string {
	notificationTablesMap := make(map[string]string)

	na.mutex.Lock()
	defer na.mutex.Unlock()

	for provider, msgs := range na.Notifications {
		providerTable := strings.Builder{}

		t := tablewriter.NewWriter(&providerTable)
		t.SetHeader([]string{"Message Type", "Notification", "Calling Object"})
		t.SetColWidth(200)
		t.SetRowLine(true)

		for _, n := range msgs {
			row := []string{string(n.Type), n.Message, convertObjectsToStr(n.CallingObjects)}
			t.Append(row)
		}

		providerTable.WriteString(fmt.Sprintf("Notifications from %v:\n", strings.ToUpper(provider)))
		t.Render()
		notificationTablesMap[provider] = providerTable.String()
	}

	return notificationTablesMap
}

func convertObjectsToStr(ob []client.Object) string {
	var sb strings.Builder

	for i, o := range ob {
		if i > 0 {
			sb.WriteString(", ")
		}
		object := o.GetObjectKind().GroupVersionKind().Kind + ": " + client.ObjectKeyFromObject(o).String()
		sb.WriteString(object)
	}

	return sb.String()
}

func NewNotification(mType MessageType, message string, callingObject ...client.Object) Notification {
	return Notification{Type: mType, Message: message, CallingObjects: callingObject}
}
