// Copyright (c) 2016 Tigera, Inc. All rights reserved.

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

package proto

import (
	"github.com/golang/glog"
	"github.com/ugorji/go/codec"
	"reflect"
)

// Envelope wraps one of the message structs in order to tag it with type info
// when marshalling/unmarshalling.
type Envelope struct {
	// Payload contains a pointer to one fo the message structs.
	Payload interface{}
}

// CodecEncodeSelf encodes this object to the give Encoder by encoding the
// message type, then the contained struct on after the other.
func (e Envelope) CodecEncodeSelf(enc *codec.Encoder) {
	payloadType := reflect.TypeOf(e.Payload)
	if payloadType.Kind() == reflect.Ptr {
		payloadType = payloadType.Elem()
	}
	tag, ok := typeToMsgType[payloadType]
	if !ok {
		glog.Fatalf("Missing type mapping for %#v", e.Payload)
	}
	enc.MustEncode(tag)
	enc.MustEncode(e.Payload)
}

// CodecDecodeSelf decodes an Envelope from the Decoder, decoding the payload
// as the correct type of message as per the message type.
func (e *Envelope) CodecDecodeSelf(dec *codec.Decoder) {
	glog.V(5).Info("Decoding Envelope")
	var msgType string
	dec.MustDecode(&msgType)
	glog.V(5).Infof("Message tag: %#v", msgType)
	t := msgTypeToType[msgType]
	if t == nil {
		glog.Fatalf("Bad message, type: %#v", msgType)
	}
	glog.V(5).Infof("Message type: %v", t)
	ptr := reflect.New(t)
	iface := ptr.Interface()
	dec.MustDecode(iface)
	e.Payload = iface
}
