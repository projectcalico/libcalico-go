// Copyright (c) 2016-2017 Tigera, Inc. All rights reserved.
//
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

package k8s

import (
	log "github.com/Sirupsen/logrus"
	"github.com/projectcalico/libcalico-go/lib/backend/api"
	"github.com/projectcalico/libcalico-go/lib/backend/model"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	k8sapi "k8s.io/client-go/pkg/api/v1"
)

func PodSyncer(callbacks api.SyncerCallbacks, kc *KubeClient) api.Syncer {
	log.Infof("Creating Pod syncer")
	return newGenericSyncer(callbacks, podSyncerFuncs{kc: kc})
}

// podSyncerFuncs implements the GenericSyncerFuncs interface and provides the set of
// functions required by a generic syncer.
type podSyncerFuncs struct {
	kc *KubeClient
}

func (f podSyncerFuncs) ListFunc(opts metav1.ListOptions) ([]model.KVPair, string, map[string]bool, error) {
	podList, err := f.kc.clientSet.Pods("").List(opts)
	if err != nil {
		return nil, "", nil, err
	}

	kvpList := []model.KVPair{}
	keys := map[string]bool{}
	c := converter{}
	for _, po := range podList.Items {
		// Ignore any updates for pods which are not ready / valid.
		if !c.isReadyCalicoPod(&po) {
			log.Debugf("Skipping pod %s/%s", po.ObjectMeta.Namespace, po.ObjectMeta.Name)
			continue
		}

		// Convert to a workload endpoint.
		wep, err := c.podToWorkloadEndpoint(&po)
		if err != nil {
			log.WithError(err).Error("Failed to convert pod to workload endpoint")
			continue
		}
		kvpList = append(kvpList, *wep)
		keys[wep.Key.String()] = true
	}

	return kvpList, podList.ObjectMeta.ResourceVersion, keys, nil
}

func (f podSyncerFuncs) WatchFunc(opts metav1.ListOptions) (watch.Interface, error) {
	return f.kc.clientSet.Pods("").Watch(opts)
}

func (f podSyncerFuncs) ParseFunc(e watch.Event) []model.KVPair {
	pod, ok := e.Object.(*k8sapi.Pod)
	if !ok {
		log.Panicf("Invalid pod event. Type: %s, Object: %+v", e.Type, e.Object)
	}
	// TODO FIX THIS
	c := converter{}

	switch e.Type {
	case watch.Deleted:
		// For deletes, the validity conditions are different.  We only care if the update
		// is not for a host-networked Pods, but don't care about IP / scheduled state.
		if c.isHostNetworked(pod) {
			log.WithField("pod", pod.Name).Debug("Pod is host networked.")
			log.Debugf("Skipping delete for pod %s/%s", pod.ObjectMeta.Namespace, pod.ObjectMeta.Name)
			return []model.KVPair{}
		}
	default:
		// Ignore add/modify updates for Pods that shouldn't be shown in the Calico API.
		// e.g host networked Pods, or Pods that don't yet have an IP address.
		if !c.isReadyCalicoPod(pod) {
			log.Debugf("Skipping add/modify for pod %s/%s", pod.ObjectMeta.Namespace, pod.ObjectMeta.Name)
			return []model.KVPair{}
		}
	}

	// Convert the received Pod into a KVPair.
	kvp, err := c.podToWorkloadEndpoint(pod)
	if err != nil {
		// If we fail to parse, then ignore this update and emit a log.
		log.WithField("error", err).Error("Failed to parse Pod event")
		return []model.KVPair{}
	}

	// We behave differently based on the event type.
	switch e.Type {
	case watch.Deleted:
		// For deletes, we need to nil out the Value part of the KVPair.
		log.Debugf("Delete for pod %s/%s", pod.ObjectMeta.Namespace, pod.ObjectMeta.Name)
		kvp.Value = nil
	}

	return []model.KVPair{&kvp}
}
