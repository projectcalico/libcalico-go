package ipam

import "github.com/projectcalico/libcalico-go/lib/net"

// Interface used to access the enabled IPPools.
type PoolAccessorInterface interface {
	GetEnabledPools(ipVersion int) ([]net.IPNet, error)
}
