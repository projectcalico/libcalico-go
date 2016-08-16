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

package etcd

import (
	goerrors "errors"
	"reflect"
	"strings"

	"time"

	"encoding/json"

	etcd "github.com/coreos/etcd/client"
	"github.com/coreos/etcd/pkg/transport"
	"github.com/golang/glog"
	"github.com/tigera/libcalico-go/lib/backend/api"
	. "github.com/tigera/libcalico-go/lib/backend/model"
	"github.com/tigera/libcalico-go/lib/errors"
	"golang.org/x/net/context"
)

var (
	etcdApplyOpts  = &etcd.SetOptions{PrevExist: etcd.PrevIgnore}
	etcdCreateOpts = &etcd.SetOptions{PrevExist: etcd.PrevNoExist}
	etcdGetOpts    = &etcd.GetOptions{Quorum: true}
	etcdListOpts   = &etcd.GetOptions{Quorum: true, Recursive: true, Sort: true}
	clientTimeout  = 30 * time.Second
)

type EtcdConfig struct {
	EtcdScheme     string `json:"etcdScheme" envconfig:"ETCD_SCHEME" default:"http"`
	EtcdAuthority  string `json:"etcdAuthority" envconfig:"ETCD_AUTHORITY" default:"127.0.0.1:2379"`
	EtcdEndpoints  string `json:"etcdEndpoints" envconfig:"ETCD_ENDPOINTS"`
	EtcdUsername   string `json:"etcdUsername" envconfig:"ETCD_USERNAME"`
	EtcdPassword   string `json:"etcdPassword" envconfig:"ETCD_PASSWORD"`
	EtcdKeyFile    string `json:"etcdKeyFile" envconfig:"ETCD_KEY_FILE"`
	EtcdCertFile   string `json:"etcdCertFile" envconfig:"ETCD_CERT_FILE"`
	EtcdCACertFile string `json:"etcdCACertFile" envconfig:"ETCD_CA_CERT_FILE"`
}

type EtcdClient struct {
	etcdClient  etcd.Client
	etcdKeysAPI etcd.KeysAPI
}

func NewEtcdClient(config *EtcdConfig) (*EtcdClient, error) {
	// Determine the location from the authority or the endpoints.  The endpoints
	// takes precedence if both are specified.
	etcdLocation := []string{}
	if config.EtcdAuthority != "" {
		etcdLocation = []string{"http://" + config.EtcdAuthority}
	}
	if config.EtcdEndpoints != "" {
		etcdLocation = strings.Split(config.EtcdEndpoints, ",")
	}

	if len(etcdLocation) == 0 {
		return nil, goerrors.New("no etcd authority or endpoints specified")
	}

	// Create the etcd client
	tls := transport.TLSInfo{
		CAFile:   config.EtcdCACertFile,
		CertFile: config.EtcdCertFile,
		KeyFile:  config.EtcdKeyFile,
	}
	transport, err := transport.NewTransport(tls, clientTimeout)
	if err != nil {
		return nil, err
	}

	cfg := etcd.Config{
		Endpoints:               etcdLocation,
		Transport:               transport,
		HeaderTimeoutPerRequest: clientTimeout,
	}

	// Plumb through the username and password if both are configured.
	if config.EtcdUsername != "" && config.EtcdPassword != "" {
		cfg.Username = config.EtcdUsername
		cfg.Password = config.EtcdPassword
	}

	client, err := etcd.New(cfg)
	if err != nil {
		return nil, err
	}
	keys := etcd.NewKeysAPI(client)

	return &EtcdClient{etcdClient: client, etcdKeysAPI: keys}, nil
}

func (c *EtcdClient) Syncer(callbacks api.SyncerCallbacks) api.Syncer {
	return newSyncer(c.etcdKeysAPI, callbacks)
}

// Create an entry in the datastore.  This errors if the entry already exists.
func (c *EtcdClient) Create(d *KVPair) (*KVPair, error) {
	return c.set(d, etcdCreateOpts)
}

// Update an existing entry in the datastore.  This errors if the entry does
// not exist.
func (c *EtcdClient) Update(d *KVPair) (*KVPair, error) {
	// If the request includes a revision, set it as the etcd previous index.
	options := etcd.SetOptions{PrevExist: etcd.PrevExist}
	if d.Revision != nil {
		options.PrevIndex = d.Revision.(uint64)
	}

	return c.set(d, &options)
}

// Set an existing entry in the datastore.  This ignores whether an entry already
// exists.
func (c *EtcdClient) Apply(d *KVPair) (*KVPair, error) {
	return c.set(d, etcdApplyOpts)
}

// Delete an entry in the datastore.  This errors if the entry does not exists.
func (c *EtcdClient) Delete(d *KVPair) error {
	key, err := d.Key.DefaultDeletePath()
	if err != nil {
		return err
	}
	etcdDeleteOpts := &etcd.DeleteOptions{Recursive: true}
	if d.Revision != nil {
		etcdDeleteOpts.PrevIndex = d.Revision.(uint64)
	}
	glog.V(2).Infof("Delete Key: %s\n", key)
	_, err = c.etcdKeysAPI.Delete(context.Background(), key, etcdDeleteOpts)
	return convertEtcdError(err, d.Key)
}

// Get an entry from the datastore.  This errors if the entry does not exist.
func (c *EtcdClient) Get(k Key) (*KVPair, error) {
	key, err := k.DefaultPath()
	if err != nil {
		return nil, err
	}
	glog.V(2).Infof("Get Key: %s\n", key)
	if results, err := c.etcdKeysAPI.Get(context.Background(), key, etcdGetOpts); err != nil {
		return nil, convertEtcdError(err, k)
	} else if object, err := ParseValue(k, []byte(results.Node.Value)); err != nil {
		return nil, err
	} else {
		if reflect.ValueOf(object).Kind() == reflect.Ptr {
			// Unwrap any pointers.
			object = reflect.ValueOf(object).Elem().Interface()
		}
		return &KVPair{Key: k, Value: object, Revision: results.Node.ModifiedIndex}, nil
	}
}

// List entries in the datastore.  This may return an empty list of there are
// no entries matching the request in the ListInterface.
func (c *EtcdClient) List(l ListInterface) ([]*KVPair, error) {
	// To list entries, we enumerate from the common root based on the supplied
	// IDs, and then filter the results.
	key := l.DefaultPathRoot()
	glog.V(2).Infof("List Key: %s\n", key)
	if results, err := c.etcdKeysAPI.Get(context.Background(), key, etcdListOpts); err != nil {
		// If the root key does not exist - that's fine, return no list entries.
		err = convertEtcdError(err, nil)
		switch err.(type) {
		case errors.ErrorResourceDoesNotExist:
			return []*KVPair{}, nil
		default:
			return nil, err
		}
	} else {
		list := filterEtcdList(results.Node, l)

		switch t := l.(type) {
		case ProfileListOptions:
			return t.ListConvert(list), nil
		}
		return list, nil
	}
}

// Set an existing entry in the datastore.  This ignores whether an entry already
// exists.
func (c *EtcdClient) set(d *KVPair, options *etcd.SetOptions) (*KVPair, error) {
	key, err := d.Key.DefaultPath()
	if err != nil {
		return nil, err
	}
	bytes, err := json.Marshal(d.Value)
	if err != nil {
		return nil, err
	}

	value := string(bytes)

	glog.V(2).Infof("Key: %#v\n", key)
	glog.V(2).Infof("Value: %s\n", value)
	if d.TTL != 0 {
		glog.V(2).Infof("TTL: %v", d.TTL)
		// Take a copy of the default options so we can set the TTL for
		// this request only.
		optionsCopy := *options
		optionsCopy.TTL = d.TTL
		options = &optionsCopy
	}
	glog.V(2).Infof("Options: %+v\n", options)
	result, err := c.etcdKeysAPI.Set(context.Background(), key, value, options)
	if err != nil {
		return nil, convertEtcdError(err, d.Key)
	}

	// Datastore object will be identical except for the modified index.
	d.Revision = result.Node.ModifiedIndex
	return d, nil
}

// Process a node returned from a list to filter results based on the List type and to
// compile and return the required results.
func filterEtcdList(n *etcd.Node, l ListInterface) []*KVPair {
	kvs := []*KVPair{}
	if n.Dir {
		for _, node := range n.Nodes {
			kvs = append(kvs, filterEtcdList(node, l)...)
		}
	} else if k := l.ParseDefaultKey(n.Key); k != nil {
		if object, err := ParseValue(k, []byte(n.Value)); err == nil {
			if reflect.ValueOf(object).Kind() == reflect.Ptr {
				// Unwrap any pointers.
				object = reflect.ValueOf(object).Elem().Interface()
			}
			do := &KVPair{Key: k, Value: object, Revision: n.ModifiedIndex}
			kvs = append(kvs, do)
		}
	}
	glog.V(2).Infof("Returning: %#v", kvs)
	return kvs
}

func convertEtcdError(err error, key Key) error {
	if err == nil {
		glog.V(2).Info("Comand completed without error")
		return nil
	}

	switch err.(type) {
	case etcd.Error:
		switch err.(etcd.Error).Code {
		case etcd.ErrorCodeTestFailed:
			glog.V(2).Info("Test failed error")
			return errors.ErrorResourceUpdateConflict{Identifier: key}
		case etcd.ErrorCodeNodeExist:
			glog.V(2).Info("Node exists error")
			return errors.ErrorResourceAlreadyExists{Err: err, Identifier: key}
		case etcd.ErrorCodeKeyNotFound:
			glog.V(2).Info("Key not found error")
			return errors.ErrorResourceDoesNotExist{Err: err, Identifier: key}
		case etcd.ErrorCodeUnauthorized:
			glog.V(2).Info("Unauthorized error")
			return errors.ErrorConnectionUnauthorized{Err: err}
		default:
			glog.V(2).Infof("Generic etcd error error: %v", err)
			return errors.ErrorDatastoreError{Err: err, Identifier: key}
		}
	default:
		glog.V(2).Infof("Unhandled error: %v", err)
		return errors.ErrorDatastoreError{Err: err, Identifier: key}
	}
}
