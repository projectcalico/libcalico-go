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

// proto contains definition structs for the interface to the Felix front-end.
//
// Each message we send or receive consists of an Envelope struct, holding
// one of the message structs as Payload.  The Envelope struct implements
// the codec Selfer API in order to insert the message type into the
// messagepack data ahead of the payload (and to use that extra type
// information to unpack the correct type of struct when decoding).
//
// Where possible, messages are intended to be idempotent.  However, for
// efficiency, IP set updates are sent as delta updates.
//
// A sample protocol flow is illustrated below.
//
//	+-----------+                                  +---------+
//	| frontend  |                                  | backend |
//	+-----------+                                  +---------+
//	      |                                             |
//	      | **Create**                                  |
//	      |-------------------------------------------->|
//	      | ---------------------\                      |
//	      |-| Start of handshake |                      |
//	      | |--------------------|                      |
//	      |                                             |
//	      | Init(Hostname, etcd config)                 |
//	      |-------------------------------------------->|
//	      |                   ------------------------\ |
//	      |                   | Connects to datastore |-|
//	      |                   |-----------------------| |
//	      |                                             |
//	      |              ConfigUpdate(global, per-host) |
//	      |<--------------------------------------------|
//	      |                                             |
//	      | ConfigResolved(logging config)              |
//	      |-------------------------------------------->|
//	      |                        -------------------\ |
//	      |                        | End of handshake |-|
//	      |                        |------------------| |
//	      |                                             |
//	      |           DatastoreStatus("wait-for-ready") |
//	      |<--------------------------------------------|
//	      |         ----------------------------------\ |
//	      |         | Starts  resync, sending updates |-|
//	      |         |---------------------------------| |
//	      |                                             |
//	      |                   DatastoreStatus("resync") |
//	      |<--------------------------------------------|
//	      |                                             |
//	      |          IPSet(Update|Delta)(Update|Remove) |
//	      |<--------------------------------------------|
//	      |                                             |
//	      |       Active(Profile|Policy)(Update|Remove) |
//	      |<--------------------------------------------|
//	      |                                             |
//	      |      (Workload|Host)Endpoint(Update|Remove) |
//	      |<--------------------------------------------|
//	      |                           ----------------\ |
//	      |                           | Finishes sync |-|
//	      |                           |---------------| |
//	      |                                             |
//	      |              ConfigUpdate(global, per-host) |
//	      |<--------------------------------------------|
//	      |                                             |
//	      |                  DatastoreStatus("in-sync") |
//	      |<--------------------------------------------|
//	      |                                             |
//	      |          IPSet(Update|Delta)(Update|Remove) |
//	      |<--------------------------------------------|
//	      |                                             |
//	      |       Active(Profile|Policy)(Update|Remove) |
//	      |<--------------------------------------------|
//	      |                                             |
//	      |      (Workload|Host)Endpoint(Update|Remove) |
//	      |<--------------------------------------------|
//	      | ------------------------------------\       |
//	      |-| Status updates (sent at any time) |       |
//	      | |-----------------------------------|       |
//	      |                                             |
//	      | FelixStatusUpdate                           |
//	      |-------------------------------------------->|
//	      |                                             |
//	      | (Workload|Host)EndpointStatus               |
//	      |-------------------------------------------->|
//	      |                                             |
package proto

// http://textart.io/sequence Source code for sequence diagram above:

var _ = `
object frontend backend
frontend->backend: **Create**
note right of frontend: Start of handshake
frontend->backend: Init(Hostname, etcd config)
note left of backend: Connects to datastore
backend->frontend: ConfigUpdate(global, per-host)
frontend->backend: ConfigResolved(logging config)
note left of backend: End of handshake

backend->frontend: DatastoreStatus("wait-for-ready")
note left of backend: Starts  resync, sending updates
backend->frontend: DatastoreStatus("resync")

backend->frontend: IPSet(Update|Delta)(Update|Remove)
backend->frontend: Active(Profile|Policy)(Update|Remove)
backend->frontend: (Workload|Host)Endpoint(Update|Remove)

note left of backend: Finishes sync
backend->frontend: ConfigUpdate(global, per-host)
backend->frontend: DatastoreStatus("in-sync")

backend->frontend: IPSet(Update|Delta)(Update|Remove)
backend->frontend: Active(Profile|Policy)(Update|Remove)
backend->frontend: (Workload|Host)Endpoint(Update|Remove)

note right of frontend: Status updates (sent at any time)
frontend->backend: FelixStatusUpdate
frontend->backend: (Workload|Host)EndpointStatus
`
