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

package proto

import (
	"fmt"
	"github.com/tigera/libcalico-go/lib/net"
	"github.com/tigera/libcalico-go/lib/numorstring"
	"reflect"
)

// Message types by struct type.
var typeToMsgType map[reflect.Type]string = map[reflect.Type]string{
	// BUG(smc) Should we shorten these for maximally compact encoding?
	reflect.TypeOf(Init{}):           "init",
	reflect.TypeOf(ConfigUpdate{}):   "config_update",
	reflect.TypeOf(ConfigResolved{}): "config_resolved",

	reflect.TypeOf(DatastoreStatus{}): "datastore_status",

	reflect.TypeOf(IPSetUpdate{}):      "ipset_update",
	reflect.TypeOf(IPSetDeltaUpdate{}): "ipset_delta",
	reflect.TypeOf(IPSetRemove{}):      "ipset_remove",

	reflect.TypeOf(ActiveProfileUpdate{}): "profile_update",
	reflect.TypeOf(ActiveProfileRemove{}): "profile_remove",

	reflect.TypeOf(ActivePolicyUpdate{}): "policy_update",
	reflect.TypeOf(ActivePolicyRemove{}): "policy_remove",

	reflect.TypeOf(WorkloadEndpointUpdate{}): "wl_ep_update",
	reflect.TypeOf(WorkloadEndpointRemove{}): "wl_ep_remove",

	reflect.TypeOf(HostEndpointUpdate{}): "host_ep_update",
	reflect.TypeOf(HostEndpointRemove{}): "host_ep_remove",
}

// Struct types by message type.
var msgTypeToType map[string]reflect.Type

func init() {
	msgTypeToType = make(map[string]reflect.Type)
	for t, tag := range typeToMsgType {
		msgTypeToType[tag] = t
	}
}

// BUG(smc) Handshake messages just document current protocol.

//

// Init is the opening message we receive from the front end.
type Init struct {
	EtcdUrls     []string `codec:"etcd_urls"`
	EtcdKeyFile  string   `codec:"etcd_key_file"`
	EtcdCertFile string   `codec:"etcd_cert_file"`
	EtcdCAFile   string   `codec:"etcd_ca_file"`

	Hostname string `codec:"hostname"`
}

// ConfigUpdate is our response with the config loaded from the datastore.
type ConfigUpdate struct {
	Global  map[string]string `codec:"global"`
	PerHost map[string]string `codec:"host"`
}

// ConfigResolved comes back from the front end to give us the resolved/parsed
// config.
type ConfigResolved struct {
	LogFile string `codec:"log_file"`
	// BUG(smc) add log severities to config resolved.
	//LogSeverityFile   string `codec:"sev_file"`
	//LogSeverityScreen string `codec:"sev_screen"`
	//LogSeveritySyslog string `codec:"sev_syslog"`
}

// DatastoreStatus is sent when the datastore status changes.
type DatastoreStatus struct {
	Status string `codec:"status"`
}

// IPSetUpdate indicates that an IP set is now actively required and should
// be set to contain the given members.
type IPSetUpdate struct {
	SetID   string   `codec:"ipset_id"`
	Members []net.IP `codec:"members"`
}

// IPSetDeltaUpdate expresses a delta change to an IP set.  The given IPs should
// be added/removed from the existing IP set.
type IPSetDeltaUpdate struct {
	SetID      string   `codec:"ipset_id"`
	AddedIPs   []net.IP `codec:"added_ips"`
	RemovedIPs []net.IP `codec:"removed_ips"`
}

// IPSetRemoved indicates that an IP set is no longer required and should be
// removed from the dataplane.
type IPSetRemove struct {
	SetID string `codec:"ipset_id"`
}

// ActiveProfileUpdate is sent when, a profile becomes active, or, an active
// profile is updated.
type ActiveProfileUpdate struct {
	Name string `codec:"name"`

	Profile Rules `codec:"profile"`
}

// ActiveProfileRemove is set when a profile is no longer active.
type ActiveProfileRemove struct {
	Name string `codec:"name"`
}

// ActivePolicyUpdate is sent when a policy becomes active, or, when an active
// policy is updated.
type ActivePolicyUpdate struct {
	Tier string `codec:"tier"`
	Name string `codec:"name"`

	Policy Rules `codec:"policy"`
}

// Rules stands in for profiles and policies.  While profiles and policies have
// different IDs, right now, the front end only needs to know about the rules they
// contain.
type Rules struct {
	InboundRules  []*Rule `codec:"inbound_rules"`
	OutboundRules []*Rule `codec:"outbound_rules"`
}

// Rule is like a backend.model.Rule, except the tag and selector matches are
// replaced with pre-calculated ipset IDs.
type Rule struct {
	Action string `codec:"action,omitempty"`

	Protocol *numorstring.Protocol `codec:"protocol,omitempty"`

	SrcNet      *net.IPNet         `codec:"src_net,omitempty"`
	SrcPorts    []numorstring.Port `codec:"src_ports,omitempty"`
	DstNet      *net.IPNet         `codec:"dst_net,omitempty"`
	DstPorts    []numorstring.Port `codec:"dst_ports,omitempty"`
	ICMPType    *int               `codec:"icmp_type,omitempty"`
	ICMPCode    *int               `codec:"icmp_code,omitempty"`
	SrcIPSetIDs []string           `codec:"src_ipsets,omitempty"`
	DstIPSetIDs []string           `codec:"dst_ipsets,omitempty"`

	NotProtocol    *numorstring.Protocol `codec:"!protocol,omitempty"`
	NotSrcNet      *net.IPNet            `codec:"!src_net,omitempty"`
	NotSrcPorts    []numorstring.Port    `codec:"!src_ports,omitempty"`
	NotDstNet      *net.IPNet            `codec:"!dst_net,omitempty"`
	NotDstPorts    []numorstring.Port    `codec:"!dst_ports,omitempty"`
	NotICMPType    *int                  `codec:"!icmp_type,omitempty"`
	NotICMPCode    *int                  `codec:"!icmp_code,omitempty"`
	NotSrcIPSetIDs []string              `codec:"!src_ipsets,omitempty"`
	NotDstIPSetIDs []string              `codec:"!dst_ipsets,omitempty"`
}

// ActivePolicyRemove is sent when a policy is no longer active.
type ActivePolicyRemove struct {
	Tier string `codec:"tier"`
	Name string `codec:"name"`
}

// WorkloadEndpointUpdate is sent when a local workload endpoint becomes
// active or gets updated.
type WorkloadEndpointUpdate struct {
	Hostname       string `codec:"hostname"`
	OrchestratorID string `codec:"orchestrator"`
	WorkloadID     string `codec:"workload_id"`
	EndpointID     string `codec:"endpoint_id"`

	Endpoint WorkloadEndpoint `codec:"endpoint"`
}

type WorkloadEndpoint struct {
	State      string      `codec:"state"`
	Name       string      `codec:"name"`
	Mac        net.MAC     `codec:"mac,omitempty"`
	ProfileIDs []string    `codec:"profile_ids,omitempty"`
	IPv4Nets   []net.IPNet `codec:"ipv4_nets,omitempty"`
	IPv6Nets   []net.IPNet `codec:"ipv6_nets,omitempty"`
	Tiers      []TierInfo  `codec:"tiers"`
}

// WorkloadEndpointRemove is sent when a local workload endpoint becomes
// inactive.
type WorkloadEndpointRemove struct {
	Hostname       string `codec:"hostname"`
	OrchestratorID string `codec:"orchestrator"`
	WorkloadID     string `codec:"workload_id"`
	EndpointID     string `codec:"endpoint_id"`
}

// HostEndpointUpdate is sent when a local host endpoint is deleted.
type HostEndpointUpdate struct {
	Hostname   string `codec:"hostname"`
	EndpointID string `codec:"endpoint_id"`

	Endpoint HostEndpoint `codec:"endpoint"`
}

type HostEndpoint struct {
	Name              string     `codec:"name,omitempty"`
	ExpectedIPv4Addrs []net.IP   `codec:"expected_ipv4_addrs,omitempty"`
	ExpectedIPv6Addrs []net.IP   `codec:"expected_ipv6_addrs,omitempty"`
	ProfileIDs        []string   `codec:"profile_ids,omitempty"`
	Tiers             []TierInfo `codec:"tiers"`
}

// HostEndpointRemove is sent when a local host endpoint is deleted.
type HostEndpointRemove struct {
	Hostname   string `codec:"hostname"`
	EndpointID string `codec:"endpoint"`
}

// TierInfo captures the information Felix needs to know about a tier; it's
// name and the ordered list of policy names that it contains.
type TierInfo struct {
	Name     string   `codec:"name"`
	Policies []string `codec:"policies"`
}

func (t TierInfo) String() string {
	return fmt.Sprintf("%v -> %v", t.Name, t.Policies)
}
