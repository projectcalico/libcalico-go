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
	"github.com/tigera/libcalico-go/felix/calc"
	"github.com/tigera/libcalico-go/felix/proto"
	"github.com/tigera/libcalico-go/felix/store"
	fapi "github.com/tigera/libcalico-go/lib/api"
	"github.com/tigera/libcalico-go/lib/backend"
	"github.com/tigera/libcalico-go/lib/backend/etcd"
	"github.com/ugorji/go/codec"
	"net"
	"os"
	"reflect"
	"strings"
	"sync"
)

const usage = `Felix backend driver.

Usage:
  felix-backend <felix-socket>`

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
	toFelix        chan interface{}
	failed         chan bool
	codecHandle    *codec.MsgpackHandle
	encoder        *codec.Encoder
	felixBufWriter *bufio.Writer
	decoder        *codec.Decoder

	dispatcher *store.Dispatcher

	addedIPs   set.Set
	removedIPs set.Set
	flushMutex sync.Mutex
}

type Startable interface {
	Start()
}

func NewFelixConnection(felixSocket net.Conn) *FelixConnection {
	// codec doesn't do any internal buffering so we need to wrap the
	// socket.
	r := bufio.NewReader(felixSocket)
	w := bufio.NewWriter(felixSocket)

	// Configure codec to return strings for map keys.
	codecHandle := &codec.MsgpackHandle{}
	codecHandle.RawToString = true
	codecHandle.MapType = reflect.TypeOf(make(map[string]interface{}))

	felixConn := &FelixConnection{
		toFelix:        make(chan interface{}),
		failed:         make(chan bool),
		codecHandle:    codecHandle,
		encoder:        codec.NewEncoder(w, codecHandle),
		felixBufWriter: w,
		decoder:        codec.NewDecoder(r, codecHandle),
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
		glog.V(3).Infof("Message from Felix: %#v", msg)

		switch msg := msg.Payload.(type) {
		case *proto.Init: // Hello message from felix
			fc.handleInitFromFelix(msg)
		default:
			glog.Warning("XXXX Unknown message from felix: ", msg)
		}
	}
}

// handleInitFromFelix() Handles the start-of-day init message from the main Felix process.
// this is the first message, which gives us the datastore configuration.
func (fc *FelixConnection) handleInitFromFelix(msg *proto.Init) {

	// Use the config to get a connection to the datastore.
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
	datastore, err := backend.NewClient(cfg)
	if err != nil {
		glog.Fatal(err)
	}

	// Respond to Felix with the etcd config.
	// BUG(SMC) Need to actually load the config from etcd.
	globalConfig := make(map[string]string)
	hostConfig := make(map[string]string)
	configMsg := proto.ConfigLoaded{
		Global:  globalConfig,
		PerHost: hostConfig,
	}
	fc.toFelix <- configMsg

	// Create the ipsets/active policy calculation pipeline, which will
	// do the dynamic calculation of ipset memberships and active policies
	// etc.
	asyncCalcGraph := calc.NewAsyncCalcGraph(msg.Hostname, fc.toFelix)

	// Create the datastore syncer, which will feed the calculation graph.
	syncer := datastore.Syncer(asyncCalcGraph)

	// Start the background processing threads.
	syncer.Start()
	asyncCalcGraph.Start()
}

func (fc *FelixConnection) sendMessagesToFelix() {
	defer func() { fc.failed <- true }()
	for {
		msg := <-fc.toFelix
		envelope := proto.Envelope{
			Payload: msg,
		}
		glog.V(3).Infof("Writing msg to felix: %#v\n", msg)
		//if glog.V(4) {
		//	bs := make([]byte, 0)
		//	enc := codec.NewEncoderBytes(&bs, fc.codecHandle)
		//	enc.Encode(envelope)
		//	dec := codec.NewDecoderBytes(bs, fc.codecHandle)
		//	msgAsMap := make(map[string]interface{})
		//	dec.Decode(msgAsMap)
		//	jsonMsg, err := json.Marshal(msgAsMap)
		//	if err == nil {
		//		glog.Infof("Dumped message: %v", string(jsonMsg))
		//	} else {
		//		glog.Infof("Failed to dump map to JSON: (%v) %v", err, msgAsMap)
		//	}
		//}
		if err := fc.encoder.Encode(envelope); err != nil {
			glog.Fatalf("Failed to send message %#v to felix: %v", msg, err)
		}
		fc.felixBufWriter.Flush()
	}
}

func (fc *FelixConnection) Start() {
	// Start background thread to read messages from Felix.
	go fc.readMessagesFromFelix()
	// And one to write to Felix.
	go fc.sendMessagesToFelix()
}

func (fc *FelixConnection) Join() {
	_ = <-fc.failed
	glog.Fatal("Background thread failed")
}
