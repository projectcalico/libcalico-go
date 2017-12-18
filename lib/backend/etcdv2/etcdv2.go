// Copyright (c) 2016-2017 Tigera, Inc. All rights reserved.

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

package etcdv2

import (
	"context"
	goerrors "errors"
	"strconv"
	"strings"
	"time"

	etcd "github.com/coreos/etcd/client"
	"github.com/coreos/etcd/pkg/transport"
	log "github.com/sirupsen/logrus"

	"github.com/projectcalico/libcalico-go/lib/apis/v1"
	"github.com/projectcalico/libcalico-go/lib/backend/api"
	"github.com/projectcalico/libcalico-go/lib/backend/model"
	"github.com/projectcalico/libcalico-go/lib/errors"
)

var (
	etcdApplyOpts        = &etcd.SetOptions{PrevExist: etcd.PrevIgnore}
	etcdCreateOpts       = &etcd.SetOptions{PrevExist: etcd.PrevNoExist}
	etcdCreateDirOpts    = &etcd.SetOptions{PrevExist: etcd.PrevNoExist, Dir: true}
	etcdDeleteEmptyOpts  = &etcd.DeleteOptions{Recursive: false, Dir: true}
	etcdGetOpts          = &etcd.GetOptions{Quorum: true}
	etcdListOpts         = &etcd.GetOptions{Quorum: true, Recursive: true, Sort: true}
	etcdListChildrenOpts = &etcd.GetOptions{Quorum: true, Recursive: false, Sort: true}
	clientTimeout        = 30 * time.Second
)

type EtcdClient struct {
	etcdClient  etcd.Client
	etcdKeysAPI etcd.KeysAPI
}

// NewEtcdClient creates a new backend etcdv2 client.  This is now only supported for
// data migration of etcv2->etcdv3.
func NewEtcdClient(config *v1.EtcdConfig) (api.Client, error) {
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
	tls := transport.TLSInfo{
		CAFile:   config.EtcdCACertFile,
		CertFile: config.EtcdCertFile,
		KeyFile:  config.EtcdKeyFile,
	}
	t, err := transport.NewTransport(tls, clientTimeout)
	if err != nil {
		return nil, err
	}

	cfg := etcd.Config{
		Endpoints:               etcdLocation,
		Transport:               t,
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

// EnsureInitialized makes sure that the etcd data is initialized for use by
// Calico.
func (c *EtcdClient) EnsureInitialized() error {
	return nil
}

func (c *EtcdClient) Syncer(callbacks api.SyncerCallbacks) api.Syncer {
	panic("Not supported")
}

// Create an entry in the datastore.  This errors if the entry already exists.
func (c *EtcdClient) Create(ctx context.Context, d *model.KVPair) (*model.KVPair, error) {
	return c.set(ctx, d, etcdCreateOpts)
}

// Update an existing entry in the datastore.  This errors if the entry does
// not exist.
func (c *EtcdClient) Update(ctx context.Context, d *model.KVPair) (*model.KVPair, error) {
	// If the request includes a revision, set it as the etcd previous index.
	options := etcd.SetOptions{PrevExist: etcd.PrevExist}
	if len(d.Revision) != 0 {
		var err error
		if options.PrevIndex, err = strconv.ParseUint(d.Revision, 10, 64); err != nil {
			return nil, err
		}
		log.Debugf("Performing CAS against etcd index: %v\n", options.PrevIndex)
	}

	return c.set(ctx, d, &options)
}

// Set an existing entry in the datastore.  This ignores whether an entry already
// exists.
func (c *EtcdClient) Apply(d *model.KVPair) (*model.KVPair, error) {
	return c.set(context.Background(), d, etcdApplyOpts)
}

// Delete an entry in the datastore.  This errors if the entry does not exists.
func (c *EtcdClient) Delete(ctx context.Context, d model.Key, revision string) (*model.KVPair, error) {
	key, err := model.KeyToDefaultDeletePath(d)
	if err != nil {
		return nil, err
	}
	etcdDeleteOpts := &etcd.DeleteOptions{Recursive: true}
	if len(revision) != 0 {
		var err error
		if etcdDeleteOpts.PrevIndex, err = strconv.ParseUint(revision, 10, 64); err != nil {
			return nil, err
		}
	}
	log.Debugf("Delete Key: %s", key)
	_, err = c.etcdKeysAPI.Delete(ctx, key, etcdDeleteOpts)
	if err != nil {
		return nil, convertEtcdError(err, d)
	}

	// If there are parents to be deleted, delete these as well provided there
	// are no more children.
	parents, err := model.KeyToDefaultDeleteParentPaths(d)
	if err != nil {
		return nil, err
	}
	for _, parent := range parents {
		log.Debugf("Delete empty Key: %s", parent)
		_, err2 := c.etcdKeysAPI.Delete(ctx, parent, etcdDeleteEmptyOpts)
		if err2 != nil {
			log.Debugf("Unable to delete parent: %v", err2)
			break
		}
	}

	return nil, convertEtcdError(err, d)
}

// Get an entry from the datastore.  This errors if the entry does not exist.
// The revision parameter is not supported for this client implementation and
// is ignored if set.
func (c *EtcdClient) Get(ctx context.Context, k model.Key, revision string) (*model.KVPair, error) {
	key, err := model.KeyToDefaultPath(k)
	if err != nil {
		return nil, err
	}

	log.Debugf("Get Key: %s", key)
	r, err := c.etcdKeysAPI.Get(ctx, key, etcdGetOpts)
	if err != nil {
		// Convert the error to our non datastore specific types
		err = convertEtcdError(err, k)

		// Older deployments with etcd may not have the Host metadata, so in the
		// event that the key does not exist, just do a get on the directory to
		// check it exists, and if so return an empty Metadata.
		if _, ok := err.(errors.ErrorResourceDoesNotExist); ok {
			if _, ok := k.(model.HostMetadataKey); ok {
				return c.getHostMetadataFromDirectory(ctx, k)
			}
		}

		return nil, err
	}

	v, err := model.ParseValue(k, []byte(r.Node.Value))
	if err != nil {
		return nil, err
	}

	return &model.KVPair{Key: k, Value: v, Revision: strconv.FormatUint(r.Node.ModifiedIndex, 10)}, nil
}

// List entries in the datastore.  This may return an empty list of there are
// no entries matching the request in the ListInterface.
// The revision parameter is not supported for this client implementation and
// is ignored if set.
func (c *EtcdClient) List(ctx context.Context, l model.ListInterface, revision string) (*model.KVPairList, error) {
	// We need to handle the listing of HostMetadata separately for two reasons:
	// -  older deployments may not have a Metadata, and instead we need to enumerate
	//    based on existence of the directory
	// -  it is not sensible to enumerate all of the endpoints, so better to enumerate
	//    the host directories and then attempt to get the metadata.
	var kvps []*model.KVPair
	var err error
	switch lt := l.(type) {
	case model.HostMetadataListOptions:
		kvps, err = c.listHostMetadata(ctx, lt)
	default:
		kvps, err = c.defaultList(ctx, l)
	}

	if err != nil {
		return nil, err
	}
	return &model.KVPairList{
		KVPairs: kvps,
	}, nil
}

func (c *EtcdClient) Watch(ctx context.Context, l model.ListInterface, revision string) (api.WatchInterface, error) {
	panic("Not supported")
}

func (c *EtcdClient) Clean() error {
	panic("Not supported")
}

// defaultList provides the default list processing.
func (c *EtcdClient) defaultList(ctx context.Context, l model.ListInterface) ([]*model.KVPair, error) {
	// To list entries, we enumerate from the common root based on the supplied
	// IDs, and then filter the results.
	key := model.ListOptionsToDefaultPathRoot(l)
	log.Debugf("List Key: %s", key)
	results, err := c.etcdKeysAPI.Get(ctx, key, etcdListOpts)
	if err != nil {
		// If the root key does not exist - that's fine, return no list entries.
		err = convertEtcdError(err, nil)
		switch err.(type) {
		case errors.ErrorResourceDoesNotExist:
			return []*model.KVPair{}, nil
		default:
			return nil, err
		}
	}

	list := filterEtcdList(results.Node, l)

	switch t := l.(type) {
	case model.ProfileListOptions:
		return t.ListConvert(list), nil
	}

	return list, nil
}

// Set an existing entry in the datastore.  This ignores whether an entry already
// exists.
func (c *EtcdClient) set(ctx context.Context, d *model.KVPair, options *etcd.SetOptions) (*model.KVPair, error) {
	logCxt := log.WithFields(log.Fields{
		"key":   d.Key,
		"value": d.Value,
		"ttl":   d.TTL,
		"rev":   d.Revision,
	})
	key, err := model.KeyToDefaultPath(d.Key)
	if err != nil {
		logCxt.WithError(err).Error("Failed to convert key to path")
		return nil, err
	}
	bytes, err := model.SerializeValue(d)
	if err != nil {
		logCxt.WithError(err).Error("Failed to serialize value")
		return nil, err
	}

	value := string(bytes)

	if d.TTL != 0 {
		logCxt.Debug("Key has TTL, copying etcd options")
		// Take a copy of the default options so we can set the TTL for
		// this request only.
		optionsCopy := *options
		optionsCopy.TTL = d.TTL
		options = &optionsCopy
	}
	logCxt.WithField("options", options).Debug("Setting KV in etcd")
	result, err := c.etcdKeysAPI.Set(ctx, key, value, options)
	if err != nil {
		// Log at debug because we don't know how serious this is.
		// Caller should log if it's actually a problem.
		logCxt.WithError(err).Debug("Set failed")
		return nil, convertEtcdError(err, d.Key)
	}

	// Datastore object will be identical except for the modified index.
	logCxt.WithField("newRev", result.Node.ModifiedIndex).Debug("Set succeeded")
	d.Revision = strconv.FormatUint(result.Node.ModifiedIndex, 10)
	return d, nil
}

// Process a node returned from a list to filter results based on the List type and to
// compile and return the required results.
func filterEtcdList(n *etcd.Node, l model.ListInterface) []*model.KVPair {
	kvs := []*model.KVPair{}
	if n.Dir {
		for _, node := range n.Nodes {
			kvs = append(kvs, filterEtcdList(node, l)...)
		}
	} else if k := l.KeyFromDefaultPath(n.Key); k != nil {
		if v, err := model.ParseValue(k, []byte(n.Value)); err == nil {
			kv := &model.KVPair{Key: k, Value: v, Revision: strconv.FormatUint(n.ModifiedIndex, 10)}
			kvs = append(kvs, kv)
		}
	}
	log.Debugf("Returning: %#v", kvs)
	return kvs
}

func convertEtcdError(err error, key model.Key) error {
	if err == nil {
		log.Debug("Command completed without error")
		return nil
	}

	switch etcdErr := err.(type) {
	case etcd.Error:
		switch etcdErr.Code {
		case etcd.ErrorCodeTestFailed:
			log.Debug("Test failed error")
			return errors.ErrorResourceUpdateConflict{Identifier: key}
		case etcd.ErrorCodeNodeExist:
			log.Debug("Node exists error")
			return errors.ErrorResourceAlreadyExists{Err: err, Identifier: key}
		case etcd.ErrorCodeKeyNotFound:
			log.Debug("Key not found error")
			return errors.ErrorResourceDoesNotExist{Err: err, Identifier: key}
		case etcd.ErrorCodeUnauthorized:
			log.Debug("Unauthorized error")
			return errors.ErrorConnectionUnauthorized{Err: err}
		default:
			log.Infof("Generic etcd error error: %v", err)
			return errors.ErrorDatastoreError{Err: err, Identifier: key}
		}
	default:
		log.Infof("Unhandled error: %v", err)
		return errors.ErrorDatastoreError{Err: err, Identifier: key}
	}
}

// getHostMetadataFromDirectory gets hosts that may not be configured with a host
// metadata (older deployments or Openstack deployments).
func (c *EtcdClient) getHostMetadataFromDirectory(ctx context.Context, k model.Key) (*model.KVPair, error) {
	// The delete path of the host metadata includes the whole of the per-host
	// felix tree, so check the existence of this tree and return and empty
	// Metadata if it exists.
	key, err := model.KeyToDefaultDeletePath(k)
	if err != nil {
		return nil, err
	}
	if _, err := c.etcdKeysAPI.Get(ctx, key, etcdGetOpts); err != nil {
		return nil, convertEtcdError(err, k)
	}

	// The node exists, so return an empty Metadata.
	kv := &model.KVPair{
		Key:   k,
		Value: &model.HostMetadata{},
	}
	return kv, nil
}

func (c *EtcdClient) listHostMetadata(ctx context.Context, l model.HostMetadataListOptions) ([]*model.KVPair, error) {
	// If the hostname is specified then just attempt to get the host,
	// returning an empty string if it does not exist.
	if l.Hostname != "" {
		log.Debug("Listing host metadata with exact key")
		hmk := model.HostMetadataKey{
			Hostname: l.Hostname,
		}

		kv, err := c.Get(ctx, hmk, "")
		if err != nil {
			err = convertEtcdError(err, nil)
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
	results, err := c.etcdKeysAPI.Get(ctx, key, etcdListChildrenOpts)
	if err != nil {
		// If the root key does not exist - that's fine, return no list entries.
		log.WithError(err).Info("Error enumerating host directories")
		err = convertEtcdError(err, nil)
		switch err.(type) {
		case errors.ErrorResourceDoesNotExist:
			return []*model.KVPair{}, nil
		default:
			return nil, err
		}
	}

	// TODO:  Since the host metadata is currently empty, we don't need
	// to perform an additional get here, but in the future when the metadata
	// may contain fields, we would need to perform a get.
	log.Debug("Parse host directories.")
	kvs := []*model.KVPair{}
	for _, n := range results.Node.Nodes {
		k := l.KeyFromDefaultPath(n.Key + "/metadata")
		if k != nil {
			kvs = append(kvs, &model.KVPair{
				Key:   k,
				Value: &model.HostMetadata{},
			})
		}
	}
	return kvs, nil
}
