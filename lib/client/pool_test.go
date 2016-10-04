package client

import (
	"net"
	"os"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/tigera/libcalico-go/lib/api"
	"github.com/tigera/libcalico-go/lib/api/unversioned"
	"github.com/tigera/libcalico-go/lib/backend/model"
	cnet "github.com/tigera/libcalico-go/lib/net"
)

var _ = Describe("Pool tests", func() {

	Describe("Pool API to KVPair tests", func() {
		convertAPItoKVPairTest()
	})
})

func convertAPItoKVPairTest() {

	table := []struct {
		description           string
		CIDR                  net.IPNet
		IPIP                  bool
		NATOut                bool
		Disabled              bool
		expectedIPIPInterface string
	}{
		{
			description: "For IPv4 with non-default values for IPIP, NATOut, IPAM",
			CIDR: net.IPNet{
				IP:   net.IPv4(10, 0, 0, 0),
				Mask: net.CIDRMask(24, 32),
			},
			IPIP:                  true,
			NATOut:                true,
			Disabled:              true,
			expectedIPIPInterface: "tunl0",
		},
		{
			description: "For IPv4 with default values for IPIP, NATOut, IPAM",
			CIDR: net.IPNet{
				IP:   net.IPv4(10, 0, 0, 0),
				Mask: net.CIDRMask(24, 32),
			},
			IPIP:                  false,
			NATOut:                false,
			Disabled:              false,
			expectedIPIPInterface: "",
		},
		{
			description: "For IPv6 with non-default bools for IPIP, NATOut, IPAM",
			CIDR: net.IPNet{
				IP:   net.ParseIP("fe80::00"),
				Mask: net.CIDRMask(120, 128),
			},
			IPIP:                  true,
			NATOut:                true,
			Disabled:              true,
			expectedIPIPInterface: "tunl0",
		},
	}

	for _, v := range table {
		It("", func() {
			pool := api.Pool{
				TypeMetadata: unversioned.TypeMetadata{
					Kind:       "pool",
					APIVersion: "v1",
				},
				Metadata: api.PoolMetadata{
					ObjectMetadata: unversioned.ObjectMetadata{},
					CIDR:           cnet.IPNet{v.CIDR},
				},
				Spec: api.PoolSpec{
					IPIP: &api.IPIPConfiguration{
						Enabled: v.IPIP,
					},
					NATOutgoing: v.NATOut,
					Disabled:    v.Disabled,
				},
			}

			client, _ := newClient("")

			p := pools{
				c: client,
			}

			out, err := p.convertAPIToKVPair(pool)

			Expect(out.Value.(model.Pool).CIDR.IPNet).To(Equal(v.CIDR))
			Expect(out.Value.(model.Pool).Masquerade).To(Equal(v.NATOut))
			Expect(out.Value.(model.Pool).Disabled).To(Equal(v.Disabled))
			Expect(out.Value.(model.Pool).IPIPInterface).To(Equal(v.expectedIPIPInterface))
			Ω(err).ShouldNot(HaveOccurred())
		})
	}
}

func newClient(cf string) (*Client, error) {
	if _, err := os.Stat(cf); err != nil {
		cf = ""
	}

	cfg, err := LoadClientConfig(cf)
	if err != nil {
		return nil, err
	}

	c, err := New(*cfg)
	if err != nil {
		return nil, err
	}

	return c, err
}
