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

package etcdv3

import (
	"context"
	goerrors "errors"
	"strconv"
	"strings"
	"time"

	etcdv3 "github.com/coreos/etcd/clientv3"
	"github.com/coreos/etcd/mvcc/mvccpb"
	"github.com/coreos/etcd/pkg/transport"
	"github.com/projectcalico/libcalico-go/lib/apiconfig"
	"github.com/projectcalico/libcalico-go/lib/backend/api"
	"github.com/projectcalico/libcalico-go/lib/backend/model"
	"github.com/projectcalico/libcalico-go/lib/errors"
	log "github.com/sirupsen/logrus"
)

var (
	clientTimeout = 30 * time.Second
)

type EtcdV3Client struct {
	etcdClient *etcdv3.Client
}

func NewEtcdV3Client(config *apiconfig.EtcdConfig) (api.Client, error) {
	// Determine the location from the authority or the endpoints.  The endpoints
	// takes precedence if both are specified.
	etcdLocation := []string{}
	if config.EtcdEndpoints != "" {
		etcdLocation = strings.Split(config.EtcdEndpoints, ",")
	}

	if len(etcdLocation) == 0 {
		return nil, goerrors.New("no etcd endpoints specified")
	}

	// Create the etcd client
	tlsInfo := &transport.TLSInfo{
		CAFile:   config.EtcdCACertFile,
		CertFile: config.EtcdCertFile,
		KeyFile:  config.EtcdKeyFile,
	}
	tls, _ := tlsInfo.ClientConfig()

	cfg := etcdv3.Config{
		Endpoints:   etcdLocation,
		TLS:         tls,
		DialTimeout: clientTimeout,
	}

	// Plumb through the username and password if both are configured.
	if config.EtcdUsername != "" && config.EtcdPassword != "" {
		cfg.Username = config.EtcdUsername
		cfg.Password = config.EtcdPassword
	}

	client, err := etcdv3.New(cfg)
	if err != nil {
		return nil, err
	}

	return &EtcdV3Client{etcdClient: client}, nil
}

// Get an entry from the datastore.  This errors if the entry does not exist.
func (c *EtcdV3Client) Get(k model.Key, revision string) (*model.KVPair, error) {
	key, err := model.KeyToDefaultPath(k)
	if err != nil {
		return nil, err
	}
	key = key + "/"

	ops := []etcdv3.OpOption{}
	if len(revision) != 0 {
		rev, err := strconv.ParseInt(revision, 10, 64)
		if err != nil {
			return nil, err
		}
		ops = append(ops, etcdv3.WithRev(rev))
	}

	log.Infof("Get Key: %s", key)
	resp, err := c.etcdClient.Get(context.Background(), key, ops...)
	if err != nil {
		return nil, errors.ErrorDatastoreError{Err: err}
	}
	if len(resp.Kvs) == 0 {
		return nil, errors.ErrorResourceDoesNotExist{Identifier: k}
	}

	kv := resp.Kvs[0]
	v, err := model.ParseValue(k, kv.Value)
	if err != nil {
		return nil, err
	}

	return &model.KVPair{
		Key:      k,
		Value:    v,
		Revision: strconv.FormatInt(kv.ModRevision, 10),
	}, nil
}

// Create an entry in the datastore.  If the entry already exists, this will return
// an ErrorResourceAlreadyExists error and the current entry.
func (c *EtcdV3Client) Create(d *model.KVPair) (*model.KVPair, error) {
	logCxt := log.WithFields(log.Fields{"key": d.Key, "value": d.Value, "ttl": d.TTL, "rev": d.Revision})
	key, value, err := getKeyValueStrings(d)
	if err != nil {
		logCxt.WithError(err).Error("failed to get key or value strings")
		return nil, err
	}
	key = key + "/"

	putOpts, err := c.getTTLOption(d)
	if err != nil {
		return nil, err
	}

	// Checking for 0 version of the key, which means it doesn't exists yet,
	// and if it does, get the current value.
	txnResp, err := c.etcdClient.Txn(context.Background()).If(
		etcdv3.Compare(etcdv3.Version(key), "=", 0),
	).Then(
		etcdv3.OpPut(key, value, putOpts...),
	).Else(
		etcdv3.OpGet(key),
	).Commit()
	if err != nil {
		logCxt.WithError(err).Debug("Create failed")
		return nil, errors.ErrorDatastoreError{Err: err}
	}

	if !txnResp.Succeeded {
		// The resource must already exist.  Extract the current value and
		// return that if possible.
		var existing *model.KVPair
		getResp := (*etcdv3.GetResponse)(txnResp.Responses[0].GetResponseRange())
		if len(getResp.Kvs) != 0 {
			if v, err := model.ParseValue(d.Key, getResp.Kvs[0].Value); err == nil {
				existing = &model.KVPair{
					Key:      d.Key,
					Value:    v,
					Revision: strconv.FormatInt(getResp.Kvs[0].ModRevision, 10),
				}
			}
		}
		return existing, errors.ErrorResourceAlreadyExists{Identifier: d.Key}
	}

	d.Revision = strconv.FormatInt(txnResp.Header.Revision, 10)

	return d, nil
}

//TODO: Maybe return the current value if the revision check fails.
// Update an entry in the datastore.  If the entry does not exist, this will return
// an ErrorResourceDoesNotExist error.  THe ResourceVersion must be specified, and if
// incorrect will return a ErrorResourceUpdateConflict error and the current entry.
func (c *EtcdV3Client) Update(d *model.KVPair) (*model.KVPair, error) {
	logCxt := log.WithFields(log.Fields{"key": d.Key, "value": d.Value, "ttl": d.TTL, "rev": d.Revision})
	key, value, err := getKeyValueStrings(d)
	if err != nil {
		logCxt.WithError(err).Error("failed to get key or value strings")
		return nil, err
	}
	key = key + "/"

	opts, err := c.getTTLOption(d)
	if err != nil {
		return nil, err
	}

	// ResourceVersion must be set for an Update.
	rev, err := strconv.ParseInt(d.Revision, 10, 64)
	if err != nil {
		return nil, err
	}
	conds := []etcdv3.Cmp{etcdv3.Compare(etcdv3.ModRevision(key), "=", rev)}

	txnResp, err := c.etcdClient.Txn(context.Background()).If(
		conds...,
	).Then(
		etcdv3.OpPut(key, value, opts...),
	).Else(
		etcdv3.OpGet(key),
	).Commit()

	if err != nil {
		logCxt.WithError(err).Debug("Update failed")
		return nil, errors.ErrorDatastoreError{Err: err}
	}

	// Etcd V3 does not return a error when compare condition fails we must verify the
	// response Succeeded field instead.  If the compare did not succeed then check for
	// a successful get to return either an UpdateConflict or a ResourceDoesNotExist error.
	if !txnResp.Succeeded {
		getResp := (*etcdv3.GetResponse)(txnResp.Responses[0].GetResponseRange())
		if len(getResp.Kvs) == 0 {
			return nil, errors.ErrorResourceDoesNotExist{Identifier: d.Key}
		}

		var existing *model.KVPair
		if v, err := model.ParseValue(d.Key, getResp.Kvs[0].Value); err == nil {
			existing = &model.KVPair{
				Key:      d.Key,
				Value:    v,
				Revision: strconv.FormatInt(getResp.Kvs[0].ModRevision, 10),
			}
		}
		return existing, errors.ErrorResourceUpdateConflict{Identifier: d.Key}
	}

	d.Revision = strconv.FormatInt(txnResp.Header.Revision, 10)

	return d, nil
}

// Apply an existing entry in the datastore.  This ignores whether an entry already
// exists.
func (c *EtcdV3Client) Apply(d *model.KVPair) (*model.KVPair, error) {
	logCxt := log.WithFields(log.Fields{"key": d.Key, "value": d.Value, "ttl": d.TTL, "rev": d.Revision})
	key, value, err := getKeyValueStrings(d)
	if err != nil {
		logCxt.WithError(err).Error("failed to get key or value strings")
		return nil, err
	}
	key = key + "/"

	resp, err := c.etcdClient.Put(context.Background(), key, value)
	if err != nil {
		return nil, errors.ErrorDatastoreError{Err: err}
	}

	d.Revision = strconv.FormatInt(resp.Header.Revision, 10)

	return d, nil
}

// Delete an entry in the datastore.  This errors if the entry does not exists.
func (c *EtcdV3Client) Delete(k model.Key, revision string) error {
	key, err := model.KeyToDefaultDeletePath(k)
	if err != nil {
		return err
	}
	key = key + "/"
	log.Debugf("Delete Key: %s", key)

	conds := []etcdv3.Cmp{}
	if len(revision) != 0 {
		rev, err := strconv.ParseInt(revision, 10, 64)
		if err != nil {
			return err
		}
		conds = append(conds, etcdv3.Compare(etcdv3.ModRevision(key), "=", rev))
	}

	if err := c.deleteKey(key, conds); err != nil {
		return err
	}

	return nil
}

func (c *EtcdV3Client) deleteKey(key string, conds []etcdv3.Cmp) error {
	txnResp, err := c.etcdClient.Txn(context.Background()).If(
		conds...,
	).Then(
		etcdv3.OpDelete(key, etcdv3.WithPrefix()),
	).Commit()
	if err != nil {
		return errors.ErrorDatastoreError{Err: err, Identifier: key}
	}

	if !txnResp.Succeeded {
		return errors.ErrorResourceUpdateConflict{Identifier: key}
	}

	delResp := txnResp.Responses[0].GetResponseDeleteRange()

	if delResp.Deleted == 0 {
		return errors.ErrorResourceDoesNotExist{Identifier: key}
	}

	return nil
}

// List entries in the datastore.  This may return an empty list of there are
// no entries matching the request in the ListInterface.
func (c *EtcdV3Client) List(l model.ListInterface, revision string) (*model.KVPairList, error) {
	// To list entries, we enumerate from the common root based on the supplied
	// IDs, and then filter the results.
	prefix := model.ListOptionsToDefaultPathRoot(l)
	prefix = prefix + "/"

	// We perform a prefix get, and may also need to perform a get based on a particular revision.
	ops := []etcdv3.OpOption{etcdv3.WithPrefix()}
	if len(revision) != 0 {
		rev, err := strconv.ParseInt(revision, 10, 64)
		if err != nil {
			return nil, err
		}
		ops = append(ops, etcdv3.WithRev(rev))
	}

	log.Infof("List prefix: %s", prefix)
	resp, err := c.etcdClient.Get(context.Background(), prefix, ops...)
	if err != nil {
		return nil, errors.ErrorDatastoreError{Err: err}
	}
	log.Infof("Found %d results", len(resp.Kvs))

	list := filterEtcdV3List(resp.Kvs, l)
	return &model.KVPairList{
		KVPairs:  list,
		Revision: strconv.FormatInt(resp.Header.Revision, 10),
	}, nil
}

// EnsureInitialized makes sure that the etcd data is initialized for use by
// Calico.
func (c *EtcdV3Client) EnsureInitialized() error {
	// Make sure the Ready flag is initialized in the datastore
	kv := &model.KVPair{
		Key:   model.ReadyFlagKey{},
		Value: true,
	}

	if _, err := c.Create(kv); err != nil {
		if _, ok := err.(errors.ErrorResourceAlreadyExists); !ok {
			log.WithError(err).Warn("Failed to set ready flag")
			return err
		}
	}

	log.Info("Ready flag is already set")
	return nil
}

// Clean removes all of the Calico data from the datastore.
func (c *EtcdV3Client) Clean() error {
	return c.deleteKey("/calico", []etcdv3.Cmp{})
}

func (c *EtcdV3Client) Syncer(callbacks api.SyncerCallbacks) api.Syncer {
	return newSyncerV3(c.etcdClient, callbacks)
}

// Process a node returned from a list to filter results based on the List type and to
// compile and return the required results.
func filterEtcdV3List(pairs []*mvccpb.KeyValue, l model.ListInterface) []*model.KVPair {
	kvs := []*model.KVPair{}
	for _, p := range pairs {
		log.Infof("Maybe filter key: %s", p.Key)
		if p.Key[len(p.Key)-1] != '/' {
			continue
		}
		if k := l.KeyFromDefaultPath(string(p.Key[:len(p.Key)-1])); k != nil {
			log.Infof("Key is valid")
			if v, err := model.ParseValue(k, p.Value); err == nil {
				log.Infof("Value is valid - storing")
				kv := &model.KVPair{Key: k, Value: v, Revision: strconv.FormatInt(p.ModRevision, 10)}
				kvs = append(kvs, kv)
			}
		}
	}

	log.Infof("Returning filtered list: %#v", kvs)
	return kvs
}

func (c *EtcdV3Client) getTTLOption(d *model.KVPair) ([]etcdv3.OpOption, error) {
	putOpts := []etcdv3.OpOption{}

	if d.TTL != 0 {
		resp, err := c.etcdClient.Lease.Grant(context.Background(), int64(d.TTL.Seconds()))
		if err != nil {
			log.WithError(err).Debug("Failed to grant a lease")
			return nil, errors.ErrorDatastoreError{Err: err}
		}

		putOpts = append(putOpts, etcdv3.WithLease(resp.ID))
	}

	return putOpts, nil
}

func getKeyValueStrings(d *model.KVPair) (string, string, error) {
	key, err := model.KeyToDefaultPath(d.Key)
	if err != nil {
		return "", "", err
	}
	bytes, err := model.SerializeValue(d)
	if err != nil {
		return "", "", err
	}

	value := string(bytes)

	return key, value, nil
}
