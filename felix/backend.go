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
	"encoding/json"
	"flag"
	"github.com/docopt/docopt-go"
	"github.com/golang/glog"
	"github.com/tigera/libcalico-go/datastructures/ip"
	"github.com/tigera/libcalico-go/felix/calc"
	"github.com/tigera/libcalico-go/felix/proto"
	"github.com/tigera/libcalico-go/felix/status"
	fapi "github.com/tigera/libcalico-go/lib/api"
	"github.com/tigera/libcalico-go/lib/backend"
	bapi "github.com/tigera/libcalico-go/lib/backend/api"
	"github.com/tigera/libcalico-go/lib/backend/etcd"
	"github.com/tigera/libcalico-go/lib/backend/model"
	"github.com/ugorji/go/codec"
	"net"
	"os"
	"reflect"
	"strings"
	"time"
)

const usage = `Felix backend driver.

Usage:
  felix-backend <felix-socket>`

var msgpackHandle = &codec.MsgpackHandle{
	RawToString: true,
	BasicHandle: codec.BasicHandle{
		DecodeOptions: codec.DecodeOptions{
			MapType: reflect.TypeOf(make(map[string]interface{})),
		},
	},
}

func main() {
	// Parse command-line args.
	arguments, err := docopt.Parse(usage, nil, true, "felix-backend 0.1", false)
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
	felixConn := NewFelixConnection(felixSocket)

	glog.Info("Starting the datastore driver")
	felixConn.Start()
	felixConn.Join()
}

type ipUpdate struct {
	ipset string
	ip    ip.Addr
}

type FelixConnection struct {
	toFelix         chan interface{}
	endpointUpdates chan interface{}
	inSync          chan bool
	failed          chan bool
	encoder         *codec.Encoder
	felixBufWriter  *bufio.Writer
	decoder         *codec.Decoder
	hostname        string
	datastore       bapi.Client
	statusReporter  *status.EndpointStatusReporter

	datastoreInSync bool
	globalConfig    map[string]string
	hostConfig      map[string]string

	firstStatusReportSent bool
}

type Startable interface {
	Start()
}

func NewFelixConnection(felixSocket net.Conn) *FelixConnection {
	// codec doesn't do any internal buffering so we need to wrap the
	// socket.
	r := bufio.NewReader(felixSocket)
	w := bufio.NewWriter(felixSocket)

	felixConn := &FelixConnection{
		toFelix:         make(chan interface{}),
		endpointUpdates: make(chan interface{}),
		inSync:          make(chan bool),
		failed:          make(chan bool),
		encoder:         codec.NewEncoder(w, msgpackHandle),
		felixBufWriter:  w,
		decoder:         codec.NewDecoder(r, msgpackHandle),
		globalConfig:    make(map[string]string),
		hostConfig:      make(map[string]string),
	}
	return felixConn
}

func (fc *FelixConnection) readMessagesFromFelix() {
	defer func() { fc.failed <- true }()
	for {
		msg := proto.Envelope{}
		err := fc.decoder.Decode(&msg)
		if err != nil {
			glog.Fatalf("Error reading from felix: %v", err)
		}
		glog.V(3).Infof("Message from Felix: %#v", msg.Payload)

		payload := msg.Payload
		switch msg := payload.(type) {
		case *proto.Init: // Hello message from felix
			fc.handleInitFromFelix(msg)
		case *proto.ConfigResolved:
			fc.handleConfigResolvedFromFelix(msg)
		case *proto.FelixStatusUpdate:
			fc.handleProcessStatusUpdateFromFelix(msg)
		case *proto.WorkloadEndpointStatus,
			*proto.WorkloadEndpointStatusRemove,
			*proto.HostEndpointStatus,
			*proto.HostEndpointStatusRemove:
			if fc.statusReporter == nil {
				glog.Fatal("Received status update before starting status reporter")
			}
			fc.endpointUpdates <- msg
		default:
			glog.Warningf("XXXX Unknown message from felix: %#v", msg)
		}
		glog.V(3).Info("Finished handling message from front-end")
	}
}

func (fc *FelixConnection) handleProcessStatusUpdateFromFelix(msg *proto.FelixStatusUpdate) {
	glog.V(3).Infof("Status update from front-end: %v", *msg)
	statusReport := model.StatusReport{
		Timestamp:     msg.Timestamp,
		UptimeSeconds: msg.UptimeSeconds,
		FirstUpdate:   !fc.firstStatusReportSent,
	}
	kv := model.KVPair{
		Key:   model.ActiveStatusReportKey{Hostname: fc.hostname},
		Value: &statusReport,
		// BUG(smc) Should honour TTL config
		TTL: 90 * time.Second,
	}
	_, err := fc.datastore.Apply(&kv)
	if err != nil {
		glog.Warningf("Failed to write status to datastore: %v", err)
	} else {
		fc.firstStatusReportSent = true
	}
	kv = model.KVPair{
		Key:   model.LastStatusReportKey{Hostname: fc.hostname},
		Value: &statusReport,
	}
	_, err = fc.datastore.Apply(&kv)
	if err != nil {
		glog.Warningf("Failed to write status to datastore: %v", err)
	}
}

// handleInitFromFelix() Handles the start-of-day init message from the main Felix process.
// this is the first message, which gives us the datastore configuration.
func (fc *FelixConnection) handleInitFromFelix(msg *proto.Init) {
	// Use the config to get a connection to the datastore.
	glog.V(1).Infof("Init message from front-end: %v", *msg)
	etcdCfg := &etcd.EtcdConfig{
		EtcdEndpoints:  strings.Join(msg.EtcdUrls, ","),
		EtcdKeyFile:    msg.EtcdKeyFile,
		EtcdCertFile:   msg.EtcdCertFile,
		EtcdCACertFile: msg.EtcdCAFile,
	}
	cfg := fapi.ClientConfig{
		BackendType:   fapi.EtcdV2,
		BackendConfig: etcdCfg,
	}
	fc.hostname = msg.Hostname

	var globalConfig, hostConfig map[string]string
	for {
		glog.V(1).Info("Connecting to datastore")
		datastore, err := backend.NewClient(cfg)
		if err != nil {
			glog.Warningf("Failed to connect to datastore: %v", err)
			time.Sleep(1 * time.Second)
			continue
		}
		fc.datastore = datastore

		glog.V(1).Info("Loading global config from dtaastore")
		kvs, err := fc.datastore.List(model.GlobalConfigListOptions{})
		if err != nil {
			glog.Warningf("Failed to load config from datastore: %v", err)
			time.Sleep(1 * time.Second)
			continue
		}
		globalConfig = make(map[string]string)
		for _, kv := range kvs {
			key := kv.Key.(model.GlobalConfigKey)
			value := kv.Value.(string)
			globalConfig[key.Name] = value
		}

		glog.V(1).Info("Loading per-host config from dtaastore")
		kvs, err = fc.datastore.List(
			model.HostConfigListOptions{Hostname: msg.Hostname})
		if err != nil {
			glog.Warningf("Failed to load config from datastore: %v", err)
			time.Sleep(1 * time.Second)
			continue
		}
		hostConfig = make(map[string]string)
		for _, kv := range kvs {
			key := kv.Key.(model.HostConfigKey)
			value := kv.Value.(string)
			hostConfig[key.Name] = value
		}
		glog.V(1).Info("Loaded config from datastore")
		break
	}

	// Respond to Felix with the etcd config.
	glog.V(1).Infof("Loaded global config from datastore: %v", globalConfig)
	glog.V(1).Infof("Loaded host config from datastore: %v", hostConfig)
	configMsg := proto.ConfigUpdate{
		Global:  globalConfig,
		PerHost: hostConfig,
	}
	fc.toFelix <- configMsg
}

func (fc *FelixConnection) handleConfigResolvedFromFelix(msg *proto.ConfigResolved) {
	glog.V(1).Infof("Config resolved message from front-end: %#v", *msg)

	fc.statusReporter = status.NewEndpointStatusReporter(
		fc.hostname, fc.endpointUpdates, fc.inSync, fc.datastore)
	fc.statusReporter.Start()

	// BUG(smc) TODO: honour the settings in the message.
	// Create the ipsets/active policy calculation pipeline, which will
	// do the dynamic calculation of ipset memberships and active policies
	// etc.
	asyncCalcGraph := calc.NewAsyncCalcGraph(fc.hostname, fc.toFelix)

	// Create the datastore syncer, which will feed the calculation graph.
	syncer := fc.datastore.Syncer(asyncCalcGraph)
	glog.V(3).Infof("Created Syncer: %#v", syncer)

	// Start the background processing threads.
	glog.V(2).Infof("Starting the datastore Syncer/processing graph")
	syncer.Start()
	asyncCalcGraph.Start()
	glog.V(2).Infof("Started the datastore Syncer/processing graph")
}

func (fc *FelixConnection) processUpdatesFromCalculationGraph() {
	defer func() { fc.failed <- true }()
	for {
		msg := <-fc.toFelix

		// Special case: we load the config at start of day before we
		// start polling the datastore. This means that there's a race
		// where the config could be changed before we start polling.
		// To close that, we rebuild our picture of the config during
		// the resync and then re-send the now-in-sync config once
		// the datastore is in-sync.
		switch msg := msg.(type) {
		case *proto.DatastoreStatus:
			if !fc.datastoreInSync && msg.Status == bapi.InSync.String() {
				fc.datastoreInSync = true
				fc.sendCachedConfigToFelix()
				fc.inSync <- true
			}
		case *calc.GlobalConfigUpdate:
			// BUG(smc) Make the calc graph do the config batching so its API can be purely felix/proto objects.
			if msg.ValueOrNil != nil {
				fc.globalConfig[msg.Name] = *msg.ValueOrNil
			} else {
				delete(fc.globalConfig, msg.Name)
			}
			if fc.datastoreInSync {
				// We're in-sync so this is a config update.
				// Tell Felix.
				fc.sendCachedConfigToFelix()
			}
			continue
		case *calc.HostConfigUpdate:
			if msg.ValueOrNil != nil {
				fc.hostConfig[msg.Name] = *msg.ValueOrNil
			} else {
				delete(fc.hostConfig, msg.Name)
			}
			if fc.datastoreInSync {
				// We're in-sync so this is a config update.
				// Tell Felix.
				fc.sendCachedConfigToFelix()
			}
			continue
		}

		fc.marshalToFelix(msg)
	}
}

func (fc *FelixConnection) sendCachedConfigToFelix() {
	msg := &proto.ConfigUpdate{
		Global:  fc.globalConfig,
		PerHost: fc.hostConfig,
	}
	fc.marshalToFelix(msg)
}

func (fc *FelixConnection) marshalToFelix(msg interface{}) {
	envelope := proto.Envelope{
		Payload: msg,
	}
	glog.V(3).Infof("Writing msg to felix: %#v\n", msg)
	if glog.V(4) {
		// For debugging purposes, dump the message to
		// messagepack; parse it as a map and dump it to JSON.
		bs := make([]byte, 0)
		enc := codec.NewEncoderBytes(&bs, msgpackHandle)
		enc.Encode(envelope)

		dec := codec.NewDecoderBytes(bs, msgpackHandle)
		var decodedType string
		msgAsMap := make(map[string]interface{})
		dec.Decode(&decodedType)
		dec.Decode(msgAsMap)
		jsonMsg, err := json.Marshal(msgAsMap)
		if err == nil {
			glog.Infof("Dumped message: %v %v", decodedType, string(jsonMsg))
		} else {
			glog.Infof("Failed to dump map to JSON: (%v) %v", err, msgAsMap)
		}
	}
	if err := fc.encoder.Encode(envelope); err != nil {
		glog.Fatalf("Failed to send message %#v to felix: %v", msg, err)
	}
	fc.felixBufWriter.Flush()
}

func (fc *FelixConnection) Start() {
	// Start background thread to read messages from Felix.
	go fc.readMessagesFromFelix()
	// And one to write to Felix.
	go fc.processUpdatesFromCalculationGraph()
}

func (fc *FelixConnection) Join() {
	_ = <-fc.failed
	glog.Fatal("Background thread failed")
}
