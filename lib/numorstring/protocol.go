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

package numorstring

import (
	"github.com/golang/glog"
	"github.com/ugorji/go/codec"
)

type Protocol struct {
	Int32OrString
}

func (p Protocol) CodecEncodeSelf(enc *codec.Encoder) {
	if p.Type == NumOrStringNum {
		enc.Encode(p.NumVal)
	} else {
		enc.Encode(p.StrVal)
	}
}

func (p *Protocol) CodecDecodeSelf(dec *codec.Decoder) {
	var v interface{}
	dec.Decode(v)
	switch v := v.(type) {
	case string:
		p.StrVal = v
		p.Type = NumOrStringString
	case float64:
		p.NumVal = int32(v)
		p.Type = NumOrStringNum
	default:
		glog.Fatalf("Unexpected type for protocol: %#v", v)
	}
}
