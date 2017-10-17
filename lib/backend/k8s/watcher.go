package k8s

import (
	"context"
	"reflect"

	"github.com/projectcalico/libcalico-go/lib/backend/api"
	"github.com/projectcalico/libcalico-go/lib/backend/model"
	"github.com/projectcalico/libcalico-go/lib/apiv2"
	"errors"
)

const (
	resultsBufSize = 100
)

// List entries in the datastore.  This may return an empty list if there are
// no entries matching the request in the ListInterface.
func (c *KubeClient) Watch(ctx context.Context, l model.ListInterface, revision string) (api.WatchInterface, error) {
	log.Debugf("Performing 'Watch' for %+v %v", l, reflect.TypeOf(l))
	switch l.(model.ResourceListOptions).Kind {
	case apiv2.KindProfile:
		return nil, errors.New("Not supported")
		//return c.listProfiles(ctx, l.(model.ResourceListOptions), revision)
	case apiv2.KindWorkloadEndpoint:
		return nil, errors.New("Not supported")
		//return c.listWorkloadEndpoints(ctx, l.(model.ResourceListOptions), revision)
	case apiv2.KindGlobalNetworkPolicy, apiv2.KindNetworkPolicy:
		return nil, errors.New("Not supported")
		//return c.listPolicies(ctx, l.(model.ResourceListOptions), revision)
	case apiv2.KindIPPool:
		return c.ipPoolClient.Watch(ctx, l, revision)
	case apiv2.KindBGPPeer:
		return c.bgpPeerClient.Watch(ctx, l, revision)
	case apiv2.KindBGPConfiguration:
		return c.bgpConfigClient.Watch(ctx, l, revision)
	case apiv2.KindFelixConfiguration:
		return c.felixConfigClient.Watch(ctx, l, revision)
	case apiv2.KindClusterInformation:
		return c.clusterInfoClient.Watch(ctx, l, revision)
	case apiv2.KindNode:
		return c.nodeClient.Watch(ctx, l, revision)
	default:
		return nil, errors.New("Not supported")
	}
}
