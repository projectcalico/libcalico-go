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
	"flag"
	"github.com/docopt/docopt-go"
	"github.com/golang/glog"
	"github.com/tigera/libcalico-go/etcd-driver/etcd"
	"github.com/tigera/libcalico-go/etcd-driver/ipsets"
	"github.com/tigera/libcalico-go/etcd-driver/store"
	"gopkg.in/vmihailenco/msgpack.v2"
	"net"
	"os"
	"strings"
)

const usage = `etcd driver.

Usage:
  etcd-driver <felix-socket>`

func main() {
	// Parse command-line args.
	arguments, err := docopt.Parse(usage, nil, true, "etcd-driver 0.1", false)
	if err != nil {
		panic(usage)
	}
	felixSckAddr := arguments["<felix-socket>"].(string)

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

	// The ipset resolver calculates the current contents of the ipsets
	// required by felix and generates events when the contents change,
	// which we then send to Felix.
	ipsetResolver := ipsets.NewResolver()
	ipsetResolver.RegisterWith(dispatcher)

	// Get an etcd driver
	datastore, err := etcd.New(felixConn, &store.DriverConfiguration{})
	felixConn.datastore = datastore

	// TODO callback functions or callback interface?
	ipsetResolver.OnIPSetAdded = felixConn.onIPSetAdded
	ipsetResolver.OnIPSetRemoved = felixConn.onIPSetRemoved
	ipsetResolver.OnIPAdded = felixConn.onIPAddedToIPSet
	ipsetResolver.OnIPRemoved = felixConn.onIPRemovedFromIPSet

	glog.Info("Starting the datastore driver")
	felixConn.Start()
	felixConn.Join()
}

type FelixConnection struct {
	toFelix    chan map[string]interface{}
	failed     chan bool
	encoder    *msgpack.Encoder
	decoder    *msgpack.Decoder
	dispatcher *store.Dispatcher
	datastore  Startable
}

type Startable interface {
	Start()
}

func NewFelixConnection(felixSocket net.Conn, disp *store.Dispatcher) *FelixConnection {
	felixConn := &FelixConnection{
		toFelix:    make(chan map[string]interface{}),
		failed:     make(chan bool),
		dispatcher: disp,
		encoder:    msgpack.NewEncoder(felixSocket),
		decoder:    msgpack.NewDecoder(felixSocket),
	}
	return felixConn
}

func (cbs *FelixConnection) onIPSetAdded(ipsetID string) {
	glog.V(2).Infof("IP set %v added; sending messsage to Felix",
		ipsetID)
	msg := map[string]interface{}{
		"type":     "ipset_added",
		"ipset_id": ipsetID,
	}
	cbs.toFelix <- msg
}

func (cbs *FelixConnection) onIPSetRemoved(ipsetID string) {
	glog.V(2).Infof("IP set %v removed; sending messsage to Felix",
		ipsetID)
	msg := map[string]interface{}{
		"type":     "ipset_removed",
		"ipset_id": ipsetID,
	}
	cbs.toFelix <- msg
}

func (cbs *FelixConnection) onIPAddedToIPSet(ipsetID string, ip string) {
	glog.V(3).Infof("IP %v added to set %v; sending messsage to Felix",
		ip, ipsetID)
	msg := map[string]interface{}{
		"type":     "ip_added",
		"ipset_id": ipsetID,
		"ip":       ip,
	}
	cbs.toFelix <- msg
}
func (cbs *FelixConnection) onIPRemovedFromIPSet(ipsetID string, ip string) {
	glog.V(3).Infof("IP %v removed from set %v; sending messsage to Felix",
		ip, ipsetID)
	msg := map[string]interface{}{
		"type":     "ip_removed",
		"ipset_id": ipsetID,
		"ip":       ip,
	}
	cbs.toFelix <- msg
}

func (cbs *FelixConnection) OnConfigLoaded(globalConfig map[string]string, hostConfig map[string]string) {
	glog.V(1).Infof("Config loaded from datastore, sending to Felix")
	msg := map[string]interface{}{
		"type":   "config_loaded",
		"global": globalConfig,
		"host":   hostConfig,
	}
	cbs.toFelix <- msg
}

func (cbs *FelixConnection) OnStatusUpdated(status store.DriverStatus) {
	statusString := "unknown"
	switch status {
	case store.WaitForDatastore:
		statusString = "wait-for-ready"
	case store.InSync:
		statusString = "in-sync"
	case store.ResyncInProgress:
		statusString = "resync"
	}
	glog.Infof("Datastore status updated to %v: %v", status, statusString)
	msg := map[string]interface{}{
		"type":   "stat",
		"status": statusString,
	}
	cbs.toFelix <- msg
}

func (cbs *FelixConnection) OnKeysUpdated(updates []store.Update) {
	glog.V(3).Infof("Sending %v key/value updates to felix", len(updates))
	for _, update := range updates {
		if len(update.Key) == 0 {
			glog.Fatal("Bug: Key/Value update had empty key")
		}

		skipFelix := cbs.dispatcher.DispatchUpdate(&update)
		if skipFelix {
			glog.V(4).Info("Skipping update to Felix")
			continue
		}

		// FIXME hard-coded hostname filter!
		if strings.Index(update.Key, "/calico/v1/host/") == 0 {
			if strings.Contains(update.Key, "/endpoint/") {
				if !strings.Contains(update.Key, "smc-ubuntu") {
					continue
				}
			}
		}

		var msg map[string]interface{}
		if update.ValueOrNil == nil {
			msg = map[string]interface{}{
				"type": "u",
				"k":    update.Key,
				"v":    nil,
			}
		} else {
			// Deref the value so that we get better diags if the
			// message is traced out.
			msg = map[string]interface{}{
				"type": "u",
				"k":    update.Key,
				"v":    *update.ValueOrNil,
			}
		}
		cbs.toFelix <- msg
	}
}

func (cbs *FelixConnection) readMessagesFromFelix() {
	defer func() { cbs.failed <- true }()
	for {
		msg, err := cbs.decoder.DecodeMap()
		if err != nil {
			panic("Error reading from felix")
		}
		glog.V(3).Infof("Message from Felix: %#v", msg)
		msgType := msg.(map[interface{}]interface{})["type"].(string)
		switch msgType {
		case "init": // Hello message from felix
			cbs.datastore.Start() // Should trigger OnConfigLoaded.
		default:
			glog.Warning("XXXX Unknown message from felix: ", msg)
		}
	}
}

func (cbs *FelixConnection) sendMessagesToFelix() {
	defer func() { cbs.failed <- true }()
	for {
		msg := <-cbs.toFelix
		glog.V(3).Infof("Writing msg to felix: %#v\n", msg)
		if err := cbs.encoder.Encode(msg); err != nil {
			panic("Failed to send message to felix")
		}
	}
}

func (cbs *FelixConnection) Start() {
	// Start background thread to read messages from Felix.
	go cbs.readMessagesFromFelix()
	// Use main thread for writing to Felix.
	go cbs.sendMessagesToFelix()
}

func (cbs *FelixConnection) Join() {
	_ = <-cbs.failed
	glog.Fatal("Background thread failed")
}
