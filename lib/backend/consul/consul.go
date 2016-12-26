// Copyright (c) 2017 Tigera, Inc. All rights reserved.

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

package consul

import (
	goerrors "errors"

	"time"

	"bytes"
	"fmt"
	log "github.com/Sirupsen/logrus"
	consulapi "github.com/hashicorp/consul/api"
	"github.com/projectcalico/libcalico-go/lib/backend/api"
	"github.com/projectcalico/libcalico-go/lib/backend/model"
	"github.com/projectcalico/libcalico-go/lib/errors"
	"regexp"
)

var (
	watchTimeout      = 30 * time.Second
	indexIsStaleError = regexp.MustCompile("failed to set key \"(.*)\", index is stale")
)

type ConsulConfig struct {
	ConsulScheme     string `json:"consulScheme" envconfig:"CONSUL_SCHEME"`
	ConsulAddress    string `json:"consulAddress" envconfig:"CONSUL_ADDRESS"`
	ConsulUsername   string `json:"consulUsername" envconfig:"CONSUL_USERNAME"`
	ConsulPassword   string `json:"consulPassword" envconfig:"CONSUL_PASSWORD"`
	ConsulToken      string `json:"consulToken" envconfig:"CONSUL_TOKEN"`
	ConsulDatacenter string `json:"consulDatacenter" envconfig:"CONSUL_DATACENTER"`
}

type ClientWrapper struct {
	Client *consulapi.Client
}

type txnError struct {
	ConsulError  *consulapi.TxnError
	ConsulOp     *consulapi.KVTxnOp
	DefaultError error
}

type setOperationKind int

const (
	create setOperationKind = iota
	replace
	update
)

type setOptions struct {
	Kind  setOperationKind
	Index uint64
}

func NewConsulClient(config *ConsulConfig) (*ClientWrapper, error) {
	auth := consulapi.HttpBasicAuth{
		Password: config.ConsulPassword,
		Username: config.ConsulUsername,
	}
	if len(config.ConsulAddress) == 0 {
		return nil, goerrors.New("no consul address provided")
	}

	cfg := consulapi.Config{
		Address:    config.ConsulAddress,
		Scheme:     config.ConsulScheme,
		Datacenter: config.ConsulDatacenter,
		Token:      config.ConsulToken,
		HttpAuth:   &auth,
		WaitTime:   watchTimeout,
	}

	client, err := consulapi.NewClient(&cfg)
	if err != nil {
		return nil, err
	}
	return &ClientWrapper{Client: client}, nil
}

func getReadyFlagPair() *model.KVPair {
	return &model.KVPair{
		Key:   model.ReadyFlagKey{},
		Value: true,
	}
}

// EnsureInitialized makes sure that the consul data is initialized for use by
// Calico.
func (c *ClientWrapper) EnsureInitialized() error {
	// Make sure the Ready flag is initialized in the datastore
	if _, err := c.Create(getReadyFlagPair()); err == nil {
		log.Info("Ready flag is now set")
	} else {
		if _, ok := err.(errors.ErrorResourceAlreadyExists); !ok {
			log.WithError(err).Warn("Failed to set ready flag")
			return err
		}
		log.Info("Ready flag is already set")
	}

	return nil
}

// EnsureCalicoNodeInitialized() performs additional initialization required
// by the calico/node components [startup/ipip-allocation/confd].  This is a
// temporary requirement until the calico/node components are updated to not
// require special consul setup, or until the global and per-node config is
// reworked to allow the node to perform the necessary updates.
func (c *ClientWrapper) EnsureCalicoNodeInitialized(node string) error {

	// The confd agent used for BIRD configuration monitors certain
	// directories and doesn't handle the non-existence of these directories
	// very well, so create the required directories.
	if err := c.ensureDirectory("/calico/v1/ipam/v4/pool"); err != nil {
		return err
	} else if err = c.ensureDirectory("/calico/v1/ipam/v6/pool"); err != nil {
		return err
	} else if err = c.ensureDirectory("/calico/bgp/v1/global/custom_filters/v4"); err != nil {
		return err
	} else if err = c.ensureDirectory("/calico/bgp/v1/global/custom_filters/v6"); err != nil {
		return err
	} else if err := c.ensureDirectory("/calico/ipam/v2/host/" + node + "/ipv4/block"); err != nil {
		return err
	} else if err = c.ensureDirectory("/calico/ipam/v2/host/" + node + "/ipv6/block"); err != nil {
		return err
	}
	return nil
}

func (c *ClientWrapper) Syncer(callbacks api.SyncerCallbacks) api.Syncer {
	return newSyncer(c.Client, callbacks)
}

// Create an entry in the datastore.  This errors if the entry already exists.
func (c *ClientWrapper) Create(d *model.KVPair) (*model.KVPair, error) {
	return c.set(d, &setOptions{Kind: create})
}

// Update an existing entry in the datastore.  This errors if the entry does
// not exist.
func (c *ClientWrapper) Update(d *model.KVPair) (*model.KVPair, error) {
	// If the request includes a revision, set it as the consul previous index.
	options := setOptions{Kind: update}
	if d.Revision != nil {
		options.Index = d.Revision.(uint64)
		log.Debugf("Performing CAS-update against consulapi index: %v\n", options.Index)
	}

	return c.set(d, &options)
}

// Set an existing entry in the datastore.  This ignores whether an entry already
// exists.
func (c *ClientWrapper) Apply(d *model.KVPair) (*model.KVPair, error) {
	return c.set(d, &setOptions{Kind: replace})
}

// Delete an entry in the datastore.  This errors if the entry does not exists.
func (c *ClientWrapper) Delete(d *model.KVPair) error {
	// Keys for recursive delete and for single delete can be different (difference is trailing slash)
	treePath, err := keyToDefaultDeleteTreePath(d.Key)
	keyPath, err := keyToDefaultPath(d.Key)
	if err != nil {
		return err
	}

	logCxt := log.WithFields(log.Fields{
		"key":     d.Key,
		"value":   d.Value,
		"ttl":     d.TTL,
		"rev":     d.Revision,
		"keypath": keyPath,
	})
	ops := consulapi.KVTxnOps{
		&consulapi.KVTxnOp{Key: keyPath, Verb: consulapi.KVDelete},
		&consulapi.KVTxnOp{Key: treePath, Verb: consulapi.KVDeleteTree},
	}

	if d.Revision != nil {
		// In Consul it is not valid to combine the -cas option with -recurse, since you are
		// deleting multiple keys under a prefix in a single operation, so we'll need a workaround
		// We do this:
		// Revision check on root key and non-recursion delete tree operation on whole tree.
		// If CAS operation will fail, whole transaction will be rollbacked, otherwise whole tree will be deleted
		ops[0].Verb = consulapi.KVDeleteCAS
		ops[0].Index = d.Revision.(uint64)
		logCxt.Debug("Delete with revision check")
	} else {
		logCxt.Debug("Delete without revision check")
	}

	logCxt.Debug("Setting KV in consul")
	kv := c.Client.KV()

	// We need to update revision on readyFlagPair, because of the way Consul works.
	// Whole explanation located in syncer.go
	readyFlagPair := getReadyFlagPair()
	readyPath, err := keyToDefaultPath(readyFlagPair.Key)
	if err != nil {
		return err
	}

	serializedValue, err := model.SerializeValue(readyFlagPair)
	if err != nil {
		return err
	}
	updateReadyOp := &consulapi.KVTxnOp{Key: readyPath, Value: serializedValue, Verb: consulapi.KVSet}
	ops = append(ops, updateReadyOp)
	ok, response, _, err := kv.Txn(ops, nil)
	if err != nil {
		logCxt.Debug("Got an error from consul")
		return convertConsulError(err, d.Key)
	}

	if !ok || len(response.Errors) > 0 {
		logCxt.Debug("Got errors in transaction")
		return convertArrayOfErrorsToError(convertTxnErrors(d.Key, ops, response.Errors))
	}

	// If there are parents to be deleted, delete these as well provided there
	// are no more children.
	// TODO: investigate if we need to do it in consul
	// As far as I can understand we need this in situation when there were keys /a/b/c and /a/b/d.
	// So, children (c and d) got deleted and we want to clean up /a/b.
	// For consul this is not a case, IMHO, because if we didn't create /a/b explicitly
	// it will return 404 on /a/b.

	// # consul kv put calico/v1/a/b/c 1
	// Success! Data written to: calico/v1/a/b/c
	// # consul kv get calico/v1/a/b
	// Error! No key exists at: calico/v1/a/b

	// But if we do create parents explicitly and logic of calico requires them to be cleaned up
	// we will need to cleanup.

	// Right now this is copy-pasted from etcd code.
	parents, err := keyToDefaultDeleteParentPaths(d.Key)
	if err != nil {
		return err
	}

	for _, parent := range parents {
		log.Debugf("Delete empty Key: %s", parent)
		_, err2 := kv.Delete(parent, nil)
		if err2 != nil {
			log.Debugf("Unable to delete parent: %s", err2)
			break
		}
	}

	return convertConsulError(err, d.Key)
}

// Get an entry from the datastore.  This errors if the entry does not exist.
func (c *ClientWrapper) Get(k model.Key) (*model.KVPair, error) {
	path, err := keyToDefaultPath(k)
	if err != nil {
		return nil, err
	}
	log.Debugf("Get Key: %s", path)
	r, meta, err := c.Client.KV().Get(path, nil)
	if err != nil {
		return nil, convertConsulError(err, k)
	}

	index := meta.LastIndex
	var value interface{}
	log.Debug("Meta", meta)
	if r != nil {
		index = r.ModifyIndex
		log.Debugf("Get Key: %s, %s, %v, %v", path, k, index, r.Value)
		value, err = model.ParseValue(k, r.Value)
		if err != nil {
			log.WithError(err).Debug("Conversion failed")
			return nil, err
		}

		return &model.KVPair{Key: k, Value: value, Revision: index}, nil
	}

	log.Debugf("Key not found error %s", path)
	return nil, errors.ErrorResourceDoesNotExist{Err: err, Identifier: k}
}

// List entries in the datastore.  This may return an empty list of there are
// no entries matching the request in the ListInterface.
func (c *ClientWrapper) List(l model.ListInterface) ([]*model.KVPair, error) {
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

// defaultList provides the default list processing.
func (c *ClientWrapper) defaultList(l model.ListInterface) ([]*model.KVPair, error) {
	// To list entries, we enumerate from the common root based on the supplied
	// IDs, and then filter the results.
	key := listOptionsToDefaultPathRoot(l)
	log.Debugf("List Key: %s", key)
	kv := c.Client.KV()

	pairs, _, err := kv.List(key, nil)

	if err != nil {
		// If the root key does not exist - that's fine, return no list entries.
		err = convertConsulError(err, nil)
		switch err.(type) {
		case errors.ErrorResourceDoesNotExist:
			return []*model.KVPair{}, nil
		default:
			return nil, err
		}
	}

	list := filterConsulList(&pairs, l)

	switch t := l.(type) {
	case model.ProfileListOptions:
		return t.ListConvert(list), nil
	}
	return list, nil
}

// Process a node returned from a list to filter results based on the List type and to
// compile and return the required results.
func filterConsulList(pairs *consulapi.KVPairs, l model.ListInterface) []*model.KVPair {
	kvs := []*model.KVPair{}

	for _, x := range *pairs {
		key := keyFromDefaultListPath(x.Key, l)
		if key == nil {
			continue
		}

		if v, err := model.ParseValue(key, x.Value); err == nil {
			kv := &model.KVPair{Key: key, Value: v, Revision: x.ModifyIndex}
			kvs = append(kvs, kv)
		}
	}
	log.Debugf("Returning: %#v", kvs)
	return kvs
}

func (c *ClientWrapper) set(d *model.KVPair, options *setOptions) (*model.KVPair, error) {
	logCxt := log.WithFields(log.Fields{
		"key":     d.Key,
		"value":   d.Value,
		"ttl":     d.TTL,
		"rev":     d.Revision,
		"options": options,
	})
	path, err := keyToDefaultPath(d.Key)
	if err != nil {
		logCxt.WithError(err).Error("Failed to convert key to path")
		return nil, err
	}

	logCxt = logCxt.WithField("path", path)

	serializedValue, err := model.SerializeValue(d)
	if err != nil {
		logCxt.WithError(err).Error("Failed to serialize value")
		return nil, err
	}

	if d.TTL != 0 {
		// Implement it via sessions & TTL
		return nil, errors.ErrorOperationNotSupported{
			Operation:  fmt.Sprintf("%s with TTL", options.Kind),
			Identifier: d.Key,
		}
	}

	ops := consulapi.KVTxnOps{}
	switch options.Kind {
	case create:
		ops = append(ops, &consulapi.KVTxnOp{
			Key:   path,
			Verb:  consulapi.KVCAS,
			Value: serializedValue,
			Index: 0,
		})
		break
	case update:
		// this get fails if there are no such key and will rollback whole transaction
		ops = append(ops, &consulapi.KVTxnOp{
			Key:  path,
			Verb: consulapi.KVGet,
		})

		if options.Index == 0 {
			ops = append(ops, &consulapi.KVTxnOp{
				Key:   path,
				Verb:  consulapi.KVSet,
				Value: serializedValue,
			})
		} else {
			ops = append(ops, &consulapi.KVTxnOp{
				Key:   path,
				Verb:  consulapi.KVCAS,
				Value: serializedValue,
				Index: options.Index,
			})
		}
		break
	case replace:
		ops = append(ops, &consulapi.KVTxnOp{
			Key:   path,
			Verb:  consulapi.KVSet,
			Value: serializedValue,
		})
		break
	default:
		log.WithField("kind", options.Kind).Error("Unsupported set operation")
		return nil, errors.ErrorOperationNotSupported{
			Operation:  fmt.Sprintf("%s", options.Kind),
			Identifier: d.Key,
		}
	}

	logCxt.Info("Setting KV in consulapi")

	ok, response, _, err := c.Client.KV().Txn(ops, nil)

	if err != nil {
		// Log at debug because we don't know how serious this is.
		// Caller should log if it's actually a problem.
		logCxt.WithError(err).Debug("Set failed, some errors")
		return nil, convertConsulError(err, d.Key)
	}

	if !ok {
		// this means that transaction was rolled back.
		// for consul this is the place for actual error detection
		txnErrs := convertTxnErrors(d.Key, ops, response.Errors)
		switch options.Kind {
		case create:
			err = convertToCreateError(d.Key, txnErrs)
		case update:
			err = convertToUpdateError(d.Key, txnErrs)
		case replace:
		default:
			err = convertArrayOfErrorsToError(txnErrs)
		}
		logCxt.WithError(err).Debug("Set failed, transaction rollbacked")
		return nil, err
	}

	// Datastore object will be identical except for the modified index.
	result := response.Results[len(response.Results)-1]
	logCxt.WithField("newRev", result.ModifyIndex).Debug("Set succeeded")

	d.Revision = result.ModifyIndex
	return d, nil
}

func convertToUpdateError(key model.Key, errs []txnError) error {
	if errs == nil || len(errs) == 0 {
		return nil
	}

	// let's check, maybe it's get error
	for _, x := range errs {
		if x.ConsulError.OpIndex == 0 {
			return errors.ErrorResourceDoesNotExist{
				Err:        goerrors.New(x.ConsulError.What),
				Identifier: key,
			}
		}
	}

	// it is not a get error, so it should be CAS error or some unknown error.
	return convertArrayOfErrorsToError(errs)
}

func convertToCreateError(key model.Key, errs []txnError) error {
	if errs == nil || len(errs) == 0 {
		return nil
	}

	if len(errs) > 1 {
		return convertArrayOfErrorsToError(errs)
	}

	if _, ok := errs[0].DefaultError.(errors.ErrorResourceUpdateConflict); ok {
		log.Debug("Node exists error")
		return errors.ErrorResourceAlreadyExists{
			Err:        goerrors.New(errs[0].ConsulError.What),
			Identifier: key,
		}
	}

	return errs[0].DefaultError
}

func convertTxnErrors(key model.Key, ops consulapi.KVTxnOps, txnErrors consulapi.TxnErrors) []txnError {
	if txnErrors == nil || len(txnErrors) == 0 {
		return nil
	}

	var buffer bytes.Buffer
	result := make([]txnError, len(txnErrors))

	buffer.WriteString("Some errors in consul:\n")
	for i, x := range txnErrors {
		result[i] = txnError{
			ConsulError: x,
			ConsulOp:    ops[i],
		}
		if matches := indexIsStaleError.FindStringSubmatch(x.What); matches != nil {
			log.Debug("Index is stale")
			result[i].DefaultError = errors.ErrorResourceUpdateConflict{
				Identifier: key,
			}
		} else {
			result[i].DefaultError = errors.ErrorDatastoreError{
				Err:        goerrors.New(fmt.Sprintf("\t[%d]: %s\n", x.OpIndex, x.What)),
				Identifier: key,
			}
		}
	}

	return result
}

func convertArrayOfErrorsToError(errors []txnError) error {
	if errors == nil || len(errors) == 0 {
		return nil
	}

	if len(errors) == 1 {
		return errors[0].DefaultError
	}

	var buffer bytes.Buffer
	for _, x := range errors {
		buffer.WriteString(fmt.Sprintf("%v\n", x.DefaultError))
	}

	return goerrors.New(buffer.String())
}

func convertConsulError(err error, key model.Key) error {
	if err == nil {
		log.Debug("Command completed without error")
		return nil
	}

	switch err.(type) {
	default:
		log.Infof("Unhandled error: %v", err)
		return errors.ErrorDatastoreError{Err: err, Identifier: key}
	}
}

func (c *ClientWrapper) listHostMetadata(l model.HostMetadataListOptions) ([]*model.KVPair, error) {
	// If the hostname is specified then just attempt to get the host,
	// returning an empty string if it does not exist.
	if l.Hostname != "" {
		log.Debug("Listing host metadata with exact key")
		hmk := model.HostMetadataKey{
			Hostname: l.Hostname,
		}

		kv, err := c.Get(hmk)
		if err == nil {
			return []*model.KVPair{kv}, nil
		}

		err = convertConsulError(err, nil)
		switch err.(type) {
		case errors.ErrorResourceDoesNotExist:
			return []*model.KVPair{}, nil
		default:
			return nil, err
		}
	}

	// No hostname specified, so enumerate the directories directly under
	// the host tree, return no entries if the host directory does not exist.
	log.Debug("Listing all host metadatas")
	key := "calico/v1/host"
	kv := c.Client.KV()
	results, _, err := kv.List(key, nil)
	if err != nil {
		// If the root key does not exist - that's fine, return no list entries.
		log.WithError(err).Info("Error enumerating host directories")
		err = convertConsulError(err, nil)
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

	logresults := []consulapi.KVPair{}
	for _, x := range results {
		logresults = append(logresults, *x)
	}

	log.Debugf("Results, %#v", logresults)
	for _, x := range results {
		k := keyFromDefaultListPath(x.Key, l)
		if k != nil {
			kvs = append(kvs, &model.KVPair{
				Key:   k,
				Value: &model.HostMetadata{},
			})
		}
	}
	return kvs, nil
}

// ensureDirectory makes sure the specified directory exists in consul.
func (c *ClientWrapper) ensureDirectory(dir string) error {
	log.WithField("Dir", dir).Debug("Ensure directory exists")
	pair := &consulapi.KVPair{
		Key:         calicoPathToConsulPath(dir),
		ModifyIndex: 0,
	}

	_, _, err := c.Client.KV().CAS(pair, nil)
	if err != nil {
		err = convertConsulError(err, nil)
		if _, ok := err.(errors.ErrorResourceAlreadyExists); !ok {
			return err
		}
	}
	return nil
}
