package namespace

import "github.com/projectcalico/libcalico-go/lib/apiv2"

func IsNamespaced(kind string) bool {
	switch kind {
	case apiv2.KindWorkloadEndpoint, apiv2.KindNetworkPolicy:
		return true
	default:
		return false
	}
}
