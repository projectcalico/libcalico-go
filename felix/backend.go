// Copyright (c) 2016 Tigera, Inc. All rights reserved.
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

package main

import (
	"bufio"
	"flag"
	"github.com/docopt/docopt-go"
	"github.com/golang/glog"
	"github.com/tigera/libcalico-go/datastructures/ip"
	"github.com/tigera/libcalico-go/datastructures/set"
	"github.com/tigera/libcalico-go/felix/endpoint"
	"github.com/tigera/libcalico-go/felix/ipsets"
	"github.com/tigera/libcalico-go/felix/store"
	fapi "github.com/tigera/libcalico-go/lib/api"
	"github.com/tigera/libcalico-go/lib/backend"
	bapi "github.com/tigera/libcalico-go/lib/backend/api"
	"github.com/tigera/libcalico-go/lib/backend/etcd"
	"github.com/tigera/libcalico-go/lib/backend/model"
	"github.com/ugorji/go/codec"
	"net"
	"os"
	"strings"
	"sync"
	"time"
)

const usage = `Felix backend driver.

Usage:
  felix-backend <felix-socket>`

func main() {
	// Parse command-line args.
	arguments, err := docopt.Parse(usage, nil, true, "etcd-driver 0.1", false)
	if err != nil {
		panic(usage)
	}
	felixSckAddr := arguments["<felix-socket>"].(string)

	// Intitialize logging.
	if os.Getenv("GLOG") != "" {
		flag.Parse()
		flag.Lookup("logtostderr").Value.Set("true")
		flag.Lookup("v").Value.Set(os.Getenv("GLOG"))
	}

	// Connect to Felix.
	glog.Info("Connecting to felix")
	felixSocket, err := net.Dial("unix", felixSckAddr)
	if err != nil {
		glog.Fatal("Failed to connect to felix")
	}
	glog.Info("Connected to Felix")

	// The dispatcher converts raw key/value pairs into typed versions and
	// fans out the updates to registered listeners.
	dispatcher := store.NewDispatcher()
	felixConn := NewFelixConnection(felixSocket, dispatcher)

	glog.Info("Starting the datastore driver")
	felixConn.Start()
	felixConn.Join()
}

type ipUpdate struct {
	ipset string
	ip    ip.Addr
}

type FelixConnection struct {
	toFelix       chan map[string]interface{}
	failed        chan bool
	encoder       *codec.Encoder
	felixBW       *bufio.Writer
	decoder       *codec.Decoder
	dispatcher    *store.Dispatcher
	syncer        Startable
	polResolver   *endpoint.PolicyResolver
	addedIPs      set.Set
	removedIPs    set.Set
	flushMutex    sync.Mutex
	endpoints     map[model.Key]interface{}
	endpointTiers map[model.Key][]TierInfo
}

type Startable interface {
	Start()
}

func NewFelixConnection(felixSocket net.Conn, disp *store.Dispatcher) *FelixConnection {
	// codec doesn't do any internal buffering so we need to wrap the
	// socket.
	r := bufio.NewReader(felixSocket)
	w := bufio.NewWriter(felixSocket)

	// Configure codec to return strings for map keys.
	codecHandle := &codec.MsgpackHandle{}
	codecHandle.RawToString = true

	felixConn := &FelixConnection{
		toFelix:       make(chan map[string]interface{}),
		failed:        make(chan bool),
		dispatcher:    disp,
		encoder:       codec.NewEncoder(w, codecHandle),
		felixBW:       w,
		decoder:       codec.NewDecoder(r, codecHandle),
		addedIPs:      set.New(),
		removedIPs:    set.New(),
		endpoints:     make(map[model.Key]interface{}),
		endpointTiers: make(map[model.Key][]TierInfo),
	}
	felixConn.polResolver = endpoint.NewPolicyResolver(felixConn)
	disp.Register(model.PolicyKey{}, felixConn.polResolver.OnUpdate)
	disp.Register(model.TierKey{}, felixConn.polResolver.OnUpdate)
	return felixConn
}

func (fc *FelixConnection) OnIPSetAdded(ipsetID string) {
	glog.V(2).Infof("IP set %v added; sending messsage to Felix",
		ipsetID)
	msg := map[string]interface{}{
		"type":     "ipset_added",
		"ipset_id": ipsetID,
	}
	fc.toFelix <- msg
}

func (fc *FelixConnection) OnIPSetRemoved(ipsetID string) {
	glog.V(2).Infof("IP set %v removed; sending messsage to Felix",
		ipsetID)
	fc.flushIPUpdates()
	msg := map[string]interface{}{
		"type":     "ipset_removed",
		"ipset_id": ipsetID,
	}
	fc.toFelix <- msg
}

func (fc *FelixConnection) OnIPAdded(ipsetID string, ip ip.Addr) {
	glog.V(3).Infof("IP %v added to set %v; updating cache",
		ip, ipsetID)
	if ip == nil {
		panic("Nil IP")
	}
	// TODO: Replace lock with go-routine?
	fc.flushMutex.Lock()
	defer fc.flushMutex.Unlock()
	upd := ipUpdate{ipsetID, ip}
	fc.addedIPs.Add(upd)
	fc.removedIPs.Discard(upd)
}
func (fc *FelixConnection) OnIPRemoved(ipsetID string, ip ip.Addr) {
	glog.V(3).Infof("IP %v removed from set %v; caching update",
		ip, ipsetID)
	if ip == nil {
		panic("Nil IP")
	}
	fc.flushMutex.Lock()
	defer fc.flushMutex.Unlock()
	upd := ipUpdate{ipsetID, ip}
	fc.addedIPs.Discard(upd)
	fc.removedIPs.Add(upd)
}

func (fc *FelixConnection) periodicallyFlush() {
	for {
		fc.flushIPUpdates()
		time.Sleep(50 * time.Millisecond)
	}
}

func (fc *FelixConnection) flushIPUpdates() {
	fc.flushMutex.Lock()
	defer fc.flushMutex.Unlock()
	if fc.addedIPs.Len() == 0 && fc.removedIPs.Len() == 0 {
		return
	}
	glog.V(3).Infof("Sending %v adds and %v IP removes to Felix",
		fc.addedIPs.Len(), fc.removedIPs.Len())
	adds := make(map[string][]string)
	fc.addedIPs.Iter(func(upd interface{}) error {
		typedUpd := upd.(ipUpdate)
		ipStr := typedUpd.ip.String()
		// FIXME: can we get a bad IP address here?
		adds[typedUpd.ipset] = append(adds[typedUpd.ipset],
			ipStr)
		return nil
	})
	removes := make(map[string][]string)
	fc.removedIPs.Iter(func(upd interface{}) error {
		typedUpd := upd.(ipUpdate)
		ipStr := typedUpd.ip.String()
		// FIXME: can we get a bad IP address here?
		removes[typedUpd.ipset] = append(removes[typedUpd.ipset],
			ipStr)
		return nil
	})
	msg := map[string]interface{}{
		"type":        "ip_updates",
		"added_ips":   adds,
		"removed_ips": removes,
	}
	fc.toFelix <- msg
	fc.addedIPs = set.New()
	fc.removedIPs = set.New()

}

func (fc *FelixConnection) OnStatusUpdated(status bapi.SyncStatus) {
	statusString := "unknown"
	switch status {
	case bapi.WaitForDatastore:
		statusString = "wait-for-ready"
	case bapi.InSync:
		statusString = "in-sync"
		fc.polResolver.InSync = true
		fc.polResolver.Flush()
	case bapi.ResyncInProgress:
		statusString = "resync"
	}
	glog.Infof("Datastore status updated to %v: %v", status, statusString)
	msg := map[string]interface{}{
		"type":   "stat",
		"status": statusString,
	}
	fc.toFelix <- msg
}

func (fc *FelixConnection) OnUpdates(updates []model.KVPair) {
	glog.V(3).Infof("Got %v key/value updates from datastore", len(updates))
	for _, update := range updates {
		update, skipFelix := fc.dispatcher.DispatchUpdate(update)
		if skipFelix {
			glog.V(4).Info("Skipping update to Felix for ",
				update.Key)
			continue
		}
		fc.SendUpdateToFelix(update)
	}
}

func (fc *FelixConnection) ParseFailed(rawKey string, rawValue *string) {
	var msg map[string]interface{}
	if rawValue == nil {
		glog.V(3).Infof("Sending KV to felix (parse failure): %v = %s", rawKey, nil)
		msg = map[string]interface{}{
			"type": "u",
			"k":    rawKey,
			"v":    nil,
		}
	} else {
		// Deref the value so that we get better diags if the
		// message is traced out.
		glog.V(3).Infof("Sending KV to felix (parse failure): %v = %s", rawKey, *rawValue)
		msg = map[string]interface{}{
			"type": "u",
			"k":    rawKey,
			"v":    rawValue,
		}
	}
	fc.toFelix <- msg
}

func (fc *FelixConnection) SendUpdateToFelix(kv model.KVPair) {
	switch key := kv.Key.(type) {
	case model.ProfileRulesKey:
		msg := map[string]interface{}{
			"type": "prof_update",
			"name": key.Name,
			"v":    kv.Value,
		}
		fc.toFelix <- msg
	case model.PolicyKey:
		msg := map[string]interface{}{
			"type": "pol_update",
			"tier": key.Tier,
			"name": key.Name,
			"v":    kv.Value,
		}
		fc.toFelix <- msg
	case model.WorkloadEndpointKey, model.HostEndpointKey:
		if kv.Value != nil {
			fc.endpoints[kv.Key] = kv.Value
		} else {
			delete(fc.endpoints, kv.Key)
		}
		fc.sendEndpointUpdate(kv.Key)
	}
}

func (fc *FelixConnection) sendEndpointUpdate(key model.Key) {
	msg := make(map[string]interface{})
	value := fc.endpoints[key]
	switch key := key.(type) {
	case model.WorkloadEndpointKey:
		msg["type"] = "wl_ep_update"
		msg["host"] = key.Hostname
		msg["orch"] = key.OrchestratorID
		msg["wl"] = key.WorkloadID
		msg["ep"] = key.EndpointID
	case model.HostEndpointKey:
		msg["type"] = "host_ep_update"
		msg["host"] = key.Hostname
		msg["ep"] = key.EndpointID
	}
	switch value := value.(type) {
	case *model.WorkloadEndpoint:
		tieredValue := workloadEndpointWithTier{}
		tieredValue.WorkloadEndpoint = *value
		tieredValue.Tiers = fc.endpointTiers[key]
		msg["v"] = tieredValue
	case *model.HostEndpoint:
		tieredValue := hostEndpointWithTier{}
		tieredValue.HostEndpoint = *value
		tieredValue.Tiers = fc.endpointTiers[key]
		msg["v"] = tieredValue
	default:
		glog.V(3).Infof("Unknown endpoint %v, %v (deletion?)", key, value)
		msg["v"] = nil
	}
	fc.toFelix <- msg
}

type workloadEndpointWithTier struct {
	model.WorkloadEndpoint
	Tiers []TierInfo `codec:"tiers"`
}

type hostEndpointWithTier struct {
	model.HostEndpoint
	Tiers []TierInfo `codec:"tiers"`
}

func (fc *FelixConnection) readMessagesFromFelix() {
	defer func() { fc.failed <- true }()
	for {
		msg := make(map[string]interface{})
		err := fc.decoder.Decode(msg)
		if err != nil {
			glog.Fatalf("Error reading from felix: %v", err)
		}
		glog.V(3).Infof("Message from Felix: %#v", msg)
		msgType := msg["type"].(string)
		switch msgType {
		case "init": // Hello message from felix
			fc.handleInitFromFelix(msg)
		default:
			glog.Warning("XXXX Unknown message from felix: ", msg)
		}
	}
}

// handleInitFromFelix() Handles the start-of-day init message from the main Felix process.
// this is the first message, which gives us the datastore configuration.
func (fc *FelixConnection) handleInitFromFelix(msg map[string]interface{}) {
	// Extract the bootstrap config from the message.
	urls := msg["etcd_urls"].([]interface{})
	urlStrs := make([]string, len(urls))
	for ii, url := range urls {
		urlStrs[ii] = url.(string)
	}
	etcdKeyFile, _ := msg["etcd_key_file"].(string)
	etcdCertFile, _ := msg["etcd_cert_file"].(string)
	etcdCACertFile, _ := msg["etcd_ca_file"].(string)
	hostname := msg["hostname"].(string)

	// Use the config to get a connection to the datastore.
	etcdCfg := &etcd.EtcdConfig{
		EtcdEndpoints:  strings.Join(urlStrs, ","),
		EtcdKeyFile:    etcdKeyFile,
		EtcdCertFile:   etcdCertFile,
		EtcdCACertFile: etcdCACertFile,
	}
	cfg := fapi.ClientConfig{
		BackendType:   fapi.EtcdV2,
		BackendConfig: etcdCfg,
	}
	datastore, err := backend.NewClient(cfg)
	if err != nil {
		glog.Fatal(err)
	}
	fc.syncer = datastore.Syncer(fc)

	// Hook up the ipset resolver to receive updates from the dispatcher.
	// The ipset resolver calculates the current contents of the ipsets
	// required by felix and generates events when the contents change,
	// which we then send to Felix.
	ipsetResolver := ipsets.NewResolver(fc, hostname, fc, fc.polResolver)
	ipsetResolver.RegisterWith(fc.dispatcher)

	// Respond to Felix with the etcd config.
	// TODO: Actually load the config from etcd.
	globalConfig := make(map[string]string)
	hostConfig := make(map[string]string)
	configMsg := map[string]interface{}{
		"type":   "config_loaded",
		"global": globalConfig,
		"host":   hostConfig,
	}
	fc.toFelix <- configMsg

	// Start the Syncer, which will send us events for datastore state
	// and changes.
	fc.syncer.Start()
}

func (fc *FelixConnection) OnEndpointTierUpdate(endpointKey model.Key, filteredTiers []endpoint.TierInfo) {
	glog.Infof("Endpoint %v now has tiers %v", endpointKey, filteredTiers)
	if len(filteredTiers) > 0 {
		tiers := make([]TierInfo, len(filteredTiers))
		for ii, ti := range filteredTiers {
			pols := make([]string, len(ti.OrderedPolicies))
			for jj, pol := range ti.OrderedPolicies {
				pols[jj] = pol.Key.Name
			}
			tiers[ii] = TierInfo{ti.Name, pols}
		}
		glog.Infof("Endpoint %v now has tiers %v", endpointKey, tiers)
		fc.endpointTiers[endpointKey] = tiers
	} else {
		delete(fc.endpointTiers, endpointKey)
	}
	fc.sendEndpointUpdate(endpointKey)
}

func (fc *FelixConnection) sendMessagesToFelix() {
	defer func() { fc.failed <- true }()
	for {
		msg := <-fc.toFelix
		glog.V(3).Infof("Writing msg to felix: %#v\n", msg)
		if err := fc.encoder.Encode(msg); err != nil {
			glog.Fatalf("Failed to send message to felix: %v", err)
		}
		fc.felixBW.Flush()
	}
}

func (fc *FelixConnection) Start() {
	// Start background thread to read messages from Felix.
	go fc.readMessagesFromFelix()
	// And one to write to Felix.
	go fc.sendMessagesToFelix()
	// And one to kick us to flush IP set updates.
	go fc.periodicallyFlush()
}

func (fc *FelixConnection) Join() {
	_ = <-fc.failed
	glog.Fatal("Background thread failed")
}

type TierInfo struct {
	Name     string   `codec:"name"`
	Policies []string `codec:"policies"`
}
