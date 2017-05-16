package etcd_test

import (
	"sort"
	"time"

	"github.com/projectcalico/libcalico-go/lib/backend/api"
	. "github.com/projectcalico/libcalico-go/lib/backend/etcd"
	"github.com/projectcalico/libcalico-go/lib/backend/model"
	"github.com/projectcalico/libcalico-go/lib/testutils"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Etcd Syncer", func() {
	// Cleaning Etcd
	testutils.CleanEtcd()

	// Init Etcd
	etcdClient, _ := NewEtcdClient(&EtcdConfig{
		EtcdEndpoints: "http://127.0.0.1:2379",
	})
	etcdClient.EnsureInitialized()

	It("creates a stream of updates", func() {
		// Syncer init
		notifyStatus := make(chan interface{})
		notifyUpdate := make(chan interface{})
		callback := NewSyncerCallbacks(notifyStatus, notifyUpdate)
		syncer := etcdClient.Syncer(callback)
		syncer.Start()

		// Give some time to syncer start watching
		time.Sleep(time.Second)

		// Create pairs
		host1 := model.KVPair{
			Key:   model.HostEndpointKey{Hostname: "host1", EndpointID: "endpoint1"},
			Value: model.HostEndpoint{Name: "host-name-1"},
		}
		host2 := model.KVPair{
			Key:   model.HostEndpointKey{Hostname: "host2", EndpointID: "endpoint2"},
			Value: model.HostEndpoint{Name: "host-name-2"},
		}
		host3 := model.KVPair{
			Key:   model.HostEndpointKey{Hostname: "host3", EndpointID: "endpoint3"},
			Value: model.HostEndpoint{Name: "host-name-3"},
		}

		etcdClient.Create(&host1)
		etcdClient.Create(&host2)
		etcdClient.Create(&host3)

		etcdClient.Delete(&host1)

		host2.Value = model.HostEndpoint{Name: "foooo"}
		etcdClient.Apply(&host2)

		updates := Updates{}
		done := false
		for {
			select {
			case _ = <-notifyStatus:
			case update := <-notifyUpdate:
				updates = append(updates, update.([]api.Update)...)
			case <-time.After(time.Second):
				done = true
			}

			if done {
				break
			}
		}

		// Sorting to make easy assertion
		sort.Sort(updates)

		Expect(updates[1].KVPair.Key).To(Equal(host1.Key))
		Expect(updates[1].UpdateType).To(Equal(api.UpdateTypeKVNew))

		Expect(updates[2].KVPair.Key).To(Equal(host2.Key))
		Expect(updates[2].UpdateType).To(Equal(api.UpdateTypeKVNew))

		Expect(updates[3].KVPair.Key).To(Equal(host3.Key))
		Expect(updates[3].UpdateType).To(Equal(api.UpdateTypeKVNew))

		Expect(updates[4].KVPair.Key).To(Equal(host1.Key))
		Expect(updates[4].UpdateType).To(Equal(api.UpdateTypeKVDeleted))

		Expect(updates[5].KVPair.Key).To(Equal(host2.Key))
		Expect(updates[5].UpdateType).To(Equal(api.UpdateTypeKVUpdated))
	})
})

func NewSyncerCallbacks(status chan interface{}, update chan interface{}) *SyncerCallbacks {
	return &SyncerCallbacks{
		NotifyStatus: status,
		NotifyUpdate: update,
	}
}

type SyncerCallbacks struct {
	NotifyUpdate chan interface{}
	NotifyStatus chan interface{}
}

func (a *SyncerCallbacks) OnStatusUpdated(status api.SyncStatus) {
	a.NotifyStatus <- status
}

func (a *SyncerCallbacks) OnUpdates(updates []api.Update) {
	a.NotifyUpdate <- updates
}

type Updates []api.Update

func (u Updates) Len() int {
	return len(u)
}
func (u Updates) Swap(i, j int) {
	u[i], u[j] = u[j], u[i]
}
func (u Updates) Less(i, j int) bool {
	return u[i].KVPair.Revision.(uint64) < u[j].KVPair.Revision.(uint64)
}
