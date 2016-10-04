package client

import (
	"net"
	"os"
	"testing"
	//. "github.com/tigera/libcalico-go/lib/client"

	// . "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	// . "github.com/onsi/ginkgo/extensions/table"

	"github.com/tigera/libcalico-go/lib/api"
	"github.com/tigera/libcalico-go/lib/api/unversioned"
	"github.com/tigera/libcalico-go/lib/backend/model"
	cnet "github.com/tigera/libcalico-go/lib/net"
)

func TestConvertAPItoKVPair(t *testing.T) {

	table := []struct {
		description           string
		CIDR                  net.IPNet
		IPIP                  bool
		NATOut                bool
		Disabled              bool
		expectedIPIPInterface string
	}{
		{
			description: "IPv4 with non-default values for IPIP, NATOut, IPAM",
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
			description: "IPv4 with default values for IPIP, NATOut, IPAM",
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
			description: "IPv6 with non-default bools for IPIP, NATOut, IPAM",
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

	}

}

// var _ = Describe("Pool", func() {

// 	_, ipnet, _ := net.ParseCIDR("192.0.2.0/24")

// 	v4Pool := api.Pool{
// 		TypeMetadata: unversioned.TypeMetadata{
// 			Kind:       "pool",
// 			APIVersion: "v1",
// 		},
// 		Metadata: api.PoolMetadata{
// 			ObjectMetadata: unversioned.ObjectMetadata{},
// 			CIDR:           cnet.IPNet{*ipnet},
// 		},
// 		Spec: api.PoolSpec{
// 			IPIP: &api.IPIPConfiguration{
// 				Enabled: true,
// 			},
// 			NATOutgoing: false,
// 			Disabled:    false,
// 		},
// 	}

// 	fmt.Println("################ test", v4Pool)
// 	client, _ := newClient("")
// 	p := pools{
// 		c: client,
// 	}

// 	out, _ := p.convertAPIToKVPair(v4Pool)

// 	fmt.Println("################ test", out.Key, out.Value.(model.Pool).CIDR)

// 	BeforeEach(func() {

// 	})

// 	// DescribeTable("Pool API to KVPair test",
// 	// 	func(input api.Pool, kv model.KVPair) {
// 	// 		if valid {
// 	// 			Expect(Validate(input)).To(BeNil(),
// 	// 				"expected value to be valid")
// 	// 		} else {
// 	// 			Expect(Validate(input)).ToNot(BeNil(),
// 	// 				"expected value to be invalid")
// 	// 		}
// 	// 	},
// 	// // Empty rule is valid, it means "allow all".
// 	// //Entry("empty rule (m)", model.Rule{}, true),

// 	// )

// 	// Describe("Categorizing book length", func() {
// 	//     Context("With more than 300 pages", func() {
// 	//         It("should be a novel", func() {
// 	//             Expect(longBook.CategoryByLength()).To(Equal("NOVEL"))
// 	//         })
// 	//     })

// 	//     Context("With fewer than 300 pages", func() {
// 	//         It("should be a short story", func() {
// 	//             Expect(shortBook.CategoryByLength()).To(Equal("SHORT STORY"))
// 	//         })
// 	//     })
// 	// })
// })

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
