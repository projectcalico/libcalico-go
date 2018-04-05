package bird

import (
	"bytes"
	"net"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
)

func init() {
	DescribeTable("Parse BIRD peer lines",
		func(line string, parsed bool, peer BGPPeer) {

			outPeer := BGPPeer{}
			outParsed := outPeer.unmarshalBIRD(line, ".")
			Expect(outParsed).To(Equal(parsed))
			Expect(outPeer).To(Equal(peer))
		},
		Entry("reject kernel", "kernel1  Kernel   master   up     2016-11-21", false, BGPPeer{}),
		Entry("reject device", "device1  Device   master   up     2016-11-21", false, BGPPeer{}),
		Entry("reject Meshd", "Meshd_172_17_8_102 BGP      master   up     2016-11-21  Established", false, BGPPeer{}),
		Entry("accept Mesh", "Mesh_172_17_8_102 BGP      master   up     2016-11-21  Established",
			true,
			BGPPeer{
				PeerIP:   "172.17.8.102",
				PeerType: "node-to-node mesh",
				State:    "up",
				Since:    "2016-11-21",
				BGPState: "Established",
				Info:     "",
			}),
		Entry("accept Node", "Node_172_17_80_102 BGP      master   up     2016-11-21  Active    Socket: error",
			true,
			BGPPeer{
				PeerIP:   "172.17.80.102",
				PeerType: "node specific",
				State:    "up",
				Since:    "2016-11-21",
				BGPState: "Active",
				Info:     "Socket: error",
			}),
		Entry("accept Global", "Global_172_17_8_133 BGP master down 2016-11-2 Failed",
			true,
			BGPPeer{
				PeerIP:   "172.17.8.133",
				PeerType: "global",
				State:    "down",
				Since:    "2016-11-2",
				BGPState: "Failed",
				Info:     "",
			}),
	)

	Describe("Test BIRD Scanner", func() {
		It("should be able to scan a table with multiple valid and invalid lines", func() {

			table := `0001 BIRD 1.5.0 ready.
2002-name     proto    table    state  since       info
1002-kernel1  Kernel   master   up     2016-11-21
 device1  Device   master   up     2016-11-21
 direct1  Direct   master   up     2016-11-21
 Mesh_172_17_8_102 BGP      master   up     2016-11-21  Established
 Global_172_17_8_103 BGP      master   up     2016-11-21  Established
 Node_172_17_8_104 BGP      master   down     2016-11-21  Failed  Socket: error
0000
We never get here
`
			expectedPeers := []BGPPeer{{
				PeerIP:   "172.17.8.102",
				PeerType: "node-to-node mesh",
				State:    "up",
				Since:    "2016-11-21",
				BGPState: "Established",
				Info:     "",
			}, {
				PeerIP:   "172.17.8.103",
				PeerType: "global",
				State:    "up",
				Since:    "2016-11-21",
				BGPState: "Established",
				Info:     "",
			}, {
				PeerIP:   "172.17.8.104",
				PeerType: "node specific",
				State:    "down",
				Since:    "2016-11-21",
				BGPState: "Failed",
				Info:     "Socket: error",
			}}
			bgpPeers, err := scanBIRDPeers(table, conn{bytes.NewBufferString(table)})

			Expect(bgpPeers).To(Equal(expectedPeers))
			Expect(err).NotTo(HaveOccurred())

			// Check we can print peers.
			// printPeers(bgpPeers)
		})

		It("should not allow a table with invalid headings", func() {
			table := `0001 BIRD 1.5.0 ready.
2002-name     proto    table    state  foo       info
1002-kernel1  Kernel   master   up     2016-11-21
 device1  Device   master   up     2016-11-21
0000
`
			_, err := scanBIRDPeers(table, conn{bytes.NewBufferString(table)})
			Expect(err).To(HaveOccurred())
		})

		It("should not allow a table with a rogue entry", func() {
			table := `0001 BIRD 1.5.0 ready.
2002-name     proto    table    state  since       info
1002-kernel1  Kernel   master   up     2016-11-21
 device1  Device   master   up     2016-11-21
9000
`
			_, err := scanBIRDPeers(table, conn{bytes.NewBufferString(table)})
			Expect(err).To(HaveOccurred())
		})
	})
}

// Implement a Mock net.Conn interface, used to emulate reading data from a
// socket.
type conn struct {
	*bytes.Buffer
}

func (c conn) Close() error {
	panic("Should not be called")
}
func (c conn) LocalAddr() net.Addr {
	panic("Should not be called")
}
func (c conn) RemoteAddr() net.Addr {
	panic("Should not be called")
}
func (c conn) SetDeadline(t time.Time) error {
	panic("Should not be called")
}
func (c conn) SetReadDeadline(t time.Time) error {
	return nil
}
func (c conn) SetWriteDeadline(t time.Time) error {
	panic("Should not be called")
}
