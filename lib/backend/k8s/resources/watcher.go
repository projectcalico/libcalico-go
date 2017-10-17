// Copyright (c) 2016-2017 Tigera, Inc. All rights reserved.

// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package resources

import (
	"sync/atomic"
	"context"

	log "github.com/sirupsen/logrus"
	kwatch "k8s.io/apimachinery/pkg/watch"

	"github.com/projectcalico/libcalico-go/lib/backend/api"
	"github.com/projectcalico/libcalico-go/lib/backend/model"
	cerrors "github.com/projectcalico/libcalico-go/lib/errors"
)

const (
	resultsBufSize = 100
)

type k8sWatcherConverter struct {
	converter  CustomK8sResourceConverter
	k8sWatcher kwatch.Interface
	context    context.Context
	cancel     context.CancelFunc
	resultChan chan api.WatchEvent
	stop       chan struct{}
	list       model.ListInterface
	terminated uint32
}

// Stop stops the watcher and releases associated resources.
// This calls through to the context cancel function.
func (crw *k8sWatcherConverter) Stop() {
	crw.cancel()
}

// ResultChan returns a channel used to receive WatchEvents.
func (crw *k8sWatcherConverter) ResultChan() <-chan api.WatchEvent {
	return crw.resultChan
}

// HasTerminated returns true when the watcher has completed termination processing.
func (crw *k8sWatcherConverter) HasTerminated() bool {
	return atomic.LoadUint32(&crw.terminated) != 0
}

// Loop to process the events stream from the underlying k8s Watcher and convert them to
// backend KVPs.
func (crw *k8sWatcherConverter) processK8sEvents() {
	defer func() {
		log.Debug("Watcher thread terminated")
		atomic.AddUint32(&crw.terminated, 1)
	}()

	for {
		select {
		case event := <-crw.k8sWatcher.ResultChan():
			e, err := crw.convertEvent(event)
			select {
			case crw.resultChan <- e:
				log.Info("Kubernetes event converted and sent to backend watcher")
				if _, ok := err.(cerrors.ErrorWatchTerminated); ok {
					log.Info("Watch terminated event")
					crw.Stop()
				}
			case <-crw.context.Done():
				log.Info("Process watcher done event during watch event in kdd client")
				return
			}
		case <-crw.context.Done(): // user cancel
			log.Info("Process watcher done event in kdd client")
			return
		}
	}
}

func (crw *k8sWatcherConverter) convertEvent(kevent kwatch.Event) (api.WatchEvent, error) {

}
