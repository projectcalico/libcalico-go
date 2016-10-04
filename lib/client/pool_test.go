package client_test

import (
	"fmt"
	"net"
	//. "github.com/tigera/libcalico-go/lib/client"

	. "github.com/onsi/ginkgo"
	//. "github.com/onsi/gomega"

	"github.com/tigera/libcalico-go/lib/api"
	"github.com/tigera/libcalico-go/lib/api/unversioned"
	cnet "github.com/tigera/libcalico-go/lib/net"
)

var _ = Describe("Pool", func() {

	var (
		v4Pool api.Pool
		// v6Pool  Book
	)

	_, ipnet, _ := net.ParseCIDR("192.0.2.0/24")
	cidr := cnet.IPNet{*ipnet}

	BeforeEach(func() {
		v4Pool = api.Pool{
			TypeMetadata: unversioned.TypeMetadata{
				Kind:       "pool",
				APIVersion: "v1",
			},
			Metadata: api.PoolMetadata{
				ObjectMetadata: unversioned.ObjectMetadata{},
				CIDR:           cidr,
			},
			Spec: api.PoolSpec{
				NATOutgoing: false,
				Disabled:    false,
			},
		}

		// shortBook = Book{
		//     Title:  "Fox In Socks",
		//     Author: "Dr. Seuss",
		//     Pages:  24,
		// }
	})
	fmt.Println(v4Pool)
	// Describe("Categorizing book length", func() {
	//     Context("With more than 300 pages", func() {
	//         It("should be a novel", func() {
	//             Expect(longBook.CategoryByLength()).To(Equal("NOVEL"))
	//         })
	//     })

	//     Context("With fewer than 300 pages", func() {
	//         It("should be a short story", func() {
	//             Expect(shortBook.CategoryByLength()).To(Equal("SHORT STORY"))
	//         })
	//     })
	// })
})
