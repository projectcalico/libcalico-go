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
	"context"
	goerrors "errors"
	"strings"

	log "github.com/Sirupsen/logrus"
	etcdv3 "github.com/coreos/etcd/clientv3"
	"github.com/coreos/etcd/mvcc/mvccpb"
	"github.com/coreos/etcd/pkg/transport"
	"github.com/projectcalico/libcalico-go/lib/backend/api"
	"github.com/projectcalico/libcalico-go/lib/backend/model"
	"github.com/projectcalico/libcalico-go/lib/errors"
)

type EtcdV3Client struct {
	etcdClient *etcdv3.Client
}

func NewEtcdV3Client(config *EtcdConfig) (*EtcdV3Client, error) {
	// Determine the location from the authority or the endpoints.  The endpoints
	// takes precedence if both are specified.
	etcdLocation := []string{}
	if config.EtcdAuthority != "" {
		etcdLocation = []string{config.EtcdScheme + "://" + config.EtcdAuthority}
	}
	if config.EtcdEndpoints != "" {
		etcdLocation = strings.Split(config.EtcdEndpoints, ",")
	}

	if len(etcdLocation) == 0 {
		return nil, goerrors.New("no etcd authority or endpoints specified")
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
func (c *EtcdV3Client) Get(k model.Key) (*model.KVPair, error) {
	key, err := model.KeyToDefaultPath(k)
	if err != nil {
		return nil, err
	}
	log.Debugf("Get Key: %s", key)
	resp, err := c.etcdClient.Get(context.Background(), key)
	if err != nil {
		return nil, errors.ErrorDatastoreError{Err: err}
	}

	// Older deployments with etcd may not have the Host metadata, so in the
	// event that the key does not exist, just do a get on the directory to
	// check it exists, and if so return an empty Metadata.
	if resp.Count == 0 {
		if _, ok := k.(model.HostMetadataKey); ok {
			return c.getHostMetadataFromDirectory(k)
		}

		return nil, errors.ErrorResourceDoesNotExist{Identifier: k}
	}

	kv := resp.Kvs[0]

	v, err := model.ParseValue(k, []byte(kv.Value))
	if err != nil {
		return nil, err
	}

	return &model.KVPair{Key: k, Value: v, Revision: kv.ModRevision}, nil
}

// Create an entry in the datastore.  This errors if the entry already exists.
func (c *EtcdV3Client) Create(d *model.KVPair) (*model.KVPair, error) {
	logCxt := log.WithFields(log.Fields{"key": d.Key, "value": d.Value, "ttl": d.TTL, "rev": d.Revision})
	return c.create(d, logCxt)
}

// Update an entry in the datastore.  This errors if the entry already exists.
func (c *EtcdV3Client) Update(d *model.KVPair) (*model.KVPair, error) {
	logCxt := log.WithFields(log.Fields{"key": d.Key, "value": d.Value, "ttl": d.TTL, "rev": d.Revision})
	return c.update(d, logCxt)
}

// Apply an existing entry in the datastore.  This ignores whether an entry already
// exists.
func (c *EtcdV3Client) Apply(d *model.KVPair) (*model.KVPair, error) {
	logCxt := log.WithFields(log.Fields{"key": d.Key, "value": d.Value, "ttl": d.TTL, "rev": d.Revision})
	return c.apply(d, logCxt)
}

// Delete an entry in the datastore.  This errors if the entry does not exists.
func (c *EtcdV3Client) Delete(d *model.KVPair) error {
	key, err := model.KeyToDefaultDeletePath(d.Key)
	if err != nil {
		return err
	}
	log.Debugf("Delete Key: %s", key)

	conds := []etcdv3.Cmp{}
	if d.Revision != nil {
		conds = append(conds, etcdv3.Compare(etcdv3.ModRevision(key), "=", d.Revision.(int64)))
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
func (c *EtcdV3Client) List(l model.ListInterface) ([]*model.KVPair, error) {
	// We need to handle the listing of HostMetadata separately for two reasons:
	// -  older deployments may not have a Metadata, and instead we need to enumerate
	//    based on existence of the directory
	// -  it is not sensible to enumerate all of the endpoints, so better to enumerate
	//    the host directories and then attempt to get the metadata.
	switch lt := l.(type) {
	case model.HostMetadataListOptions:
		return c.listHostMetadata(lt)
	default:
		return c.defaultList(l)
	}
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

func (c *EtcdV3Client) EnsureCalicoNodeInitialized(node string) error {
	// No need to do anything here when using Etcd v3
	log.WithField("Node", node).Info("Ensuring node is initialized")
	return nil
}

func (c *EtcdV3Client) listHostMetadata(l model.HostMetadataListOptions) ([]*model.KVPair, error) {
	// If the hostname is specified then just attempt to get the host,
	// returning an empty string if it does not exist.
	if l.Hostname != "" {
		log.Debug("Listing host metadata with exact key")
		hmk := model.HostMetadataKey{Hostname: l.Hostname}
		kv, err := c.Get(hmk)
		if err != nil {
			switch err.(type) {
			case errors.ErrorResourceDoesNotExist:
				return []*model.KVPair{}, nil
			default:
				return nil, err
			}
		}

		return []*model.KVPair{kv}, nil
	}

	// No hostname specified, so enumerate the directories directly under
	// the host tree, return no entries if the host directory does not exist.
	log.Debug("Listing all host metadatas")
	key := "/calico/v1/host"
	resp, err := c.etcdClient.Get(context.Background(), key, etcdv3.WithPrefix())
	if err != nil {
		return nil, errors.ErrorDatastoreError{Err: err}
	}

	// TODO:  Since the host metadata is currently empty, we don't need
	// to perform an additional get here, but in the future when the metadata
	// may contain fields, we would need to perform a get.
	log.Debug("Parse host directories.")
	kvs := []*model.KVPair{}
	for _, n := range resp.Kvs {
		k := l.KeyFromDefaultPath(string(n.Key))
		if k != nil {
			kvs = append(kvs, &model.KVPair{
				Key:   k,
				Value: &model.HostMetadata{},
			})
		}
	}

	return kvs, nil
}

// defaultList provides the default list processing.
func (c *EtcdV3Client) defaultList(l model.ListInterface) ([]*model.KVPair, error) {
	// To list entries, we enumerate from the common root based on the supplied
	// IDs, and then filter the results.
	key := model.ListOptionsToDefaultPathRoot(l)
	log.Debugf("List Key: %s", key)
	resp, err := c.etcdClient.Get(context.Background(), key, etcdv3.WithPrefix())
	if err != nil {
		return nil, errors.ErrorDatastoreError{Err: err}
	}

	list := filterEtcdV3List(resp.Kvs, l)

	switch t := l.(type) {
	case model.ProfileListOptions:
		return t.ListConvert(list), nil
	}

	return list, nil
}

// Process a node returned from a list to filter results based on the List type and to
// compile and return the required results.
func filterEtcdV3List(pairs []*mvccpb.KeyValue, l model.ListInterface) []*model.KVPair {
	kvs := []*model.KVPair{}
	for _, p := range pairs {
		if k := l.KeyFromDefaultPath(string(p.Key)); k != nil {
			if v, err := model.ParseValue(k, p.Value); err == nil {
				kv := &model.KVPair{Key: k, Value: v, Revision: p.ModRevision}
				kvs = append(kvs, kv)
			}
		}
	}

	log.Debugf("Returning filtered list: %#v", kvs)
	return kvs
}

func (c *EtcdV3Client) apply(d *model.KVPair, log *log.Entry) (*model.KVPair, error) {
	key, value, err := getKeyValueStrings(d)
	if err != nil {
		log.WithError(err).Error("failed to get key or value strings")
		return nil, err
	}

	resp, err := c.etcdClient.Put(context.Background(), key, value)
	if err != nil {
		return nil, errors.ErrorDatastoreError{Err: err}
	}

	d.Revision = resp.Header.Revision

	return d, nil
}

func (c *EtcdV3Client) update(d *model.KVPair, log *log.Entry) (*model.KVPair, error) {
	key, value, err := getKeyValueStrings(d)
	if err != nil {
		log.WithError(err).Error("failed to get key or value strings")
		return nil, err
	}

	opts, err := c.getTTLOption(d)
	if err != nil {
		return nil, err
	}

	// Checking key existence
	if _, err := c.Get(d.Key); err != nil {
		return nil, err
	}

	conds := []etcdv3.Cmp{}
	if d.Revision != nil {
		conds = append(conds, etcdv3.Compare(etcdv3.ModRevision(key), "=", d.Revision.(int64)))
	}

	txnResp, err := c.etcdClient.Txn(context.Background()).If(conds...).Then(
		etcdv3.OpPut(key, value, opts...),
	).Commit()

	if err != nil {
		log.WithError(err).Debug("Update failed")
		return nil, errors.ErrorDatastoreError{Err: err}
	}

	// Etcd V3 does not return a error when compare condition fails
	// we must verify the response Succeeded field instead
	if !txnResp.Succeeded {
		return nil, errors.ErrorResourceUpdateConflict{Identifier: d.Key}
	}

	d.Revision = txnResp.Header.Revision

	return d, nil
}

func (c *EtcdV3Client) create(d *model.KVPair, log *log.Entry) (*model.KVPair, error) {
	key, value, err := getKeyValueStrings(d)
	if err != nil {
		log.WithError(err).Error("failed to get key or value strings")
		return nil, err
	}

	putOpts, err := c.getTTLOption(d)
	if err != nil {
		return nil, err
	}

	// Checking for 0 version of the key, which means it doesn't exists yet
	cond := etcdv3.Compare(etcdv3.Version(key), "=", 0)
	req := etcdv3.OpPut(key, value, putOpts...)
	resp, err := c.etcdClient.Txn(context.Background()).If(cond).Then(req).Commit()
	if err != nil {
		log.WithError(err).Debug("Create failed")
		return nil, errors.ErrorDatastoreError{Err: err}
	}

	if !resp.Succeeded {
		return nil, errors.ErrorResourceAlreadyExists{
			Err:        goerrors.New("resource already exists"),
			Identifier: d.Key,
		}
	}

	d.Revision = resp.Header.Revision

	return d, nil
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

// getHostMetadataFromDirectory gets hosts that may not be configured with a host
// metadata (older deployments or Openstack deployments).
func (c *EtcdV3Client) getHostMetadataFromDirectory(k model.Key) (*model.KVPair, error) {
	// The delete path of the host metadata includes the whole of the per-host
	// felix tree, so check the existence of this tree and return and empty
	// Metadata if it exists.
	key, err := model.KeyToDefaultDeletePath(k)
	if err != nil {
		return nil, err
	}

	resp, err := c.etcdClient.Get(context.Background(), key)
	if err != nil {
		return nil, errors.ErrorDatastoreError{Err: err}
	}

	if resp.Count == 0 {
		return nil, errors.ErrorResourceDoesNotExist{Identifier: k}
	}

	// The node exists, so return an empty Metadata.
	kv := &model.KVPair{
		Key:   k,
		Value: &model.HostMetadata{},
	}
	return kv, nil
}

func (c *EtcdV3Client) Syncer(callbacks api.SyncerCallbacks) api.Syncer {
	return newSyncerV3(c.etcdClient, callbacks)
}
