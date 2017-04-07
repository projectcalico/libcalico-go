package etcd_test

import (
	"context"
	"github.com/coreos/etcd/client"
	"strings"
	"sync"
)

func newMockEtcd() *mockEtcd {
	return &mockEtcd{
		cond: sync.Cond{
			L: &sync.Mutex{},
		},
		states: []*client.Node{
			{
				Key: "/",
				CreatedIndex: 0,
				ModifiedIndex: 0,
				Dir: true,
			},
		},
		deltaEvents: []*client.Response{
			nil,
		},
		clusterID: "floof",
	}
}

type mockEtcd struct {
	cond sync.Cond

	// states contains each complete state of the datastore in turn, allowing us to return
	// stale snapshots, for example.
	states      []*client.Node
	// deltaEvents contains the delta between each state, stored as a watcher Response object.
	deltaEvents []*client.Response

	clusterID string
}

func (e *mockEtcd) currentIndex() int {
	return len(e.states)-1
}

func (e *mockEtcd) nextIndex() int {
	return len(e.states)
}

func (e *mockEtcd) set(key, value string) {
	e.cond.L.Lock()
	defer e.cond.L.Unlock()

	parts := strings.Split(key, '/')
	newRoot := *(e.states[len(e.states)-1])
	node := &newRoot
	partLoop: for _, part := range parts {
		if part == "" {
			continue
		}
		// Node itself is already a copy, copy the slice containing its children so we can
		// modify it.
		childrenCopy := make(client.Nodes, len(node.Nodes))
		copy(childrenCopy, node.Nodes)
		node.Nodes = childrenCopy

		// Look for the correct child node to modify
		for i, child := range node.Nodes {
			childKeyParts := strings.Split(child.Key, '/')
			if childKeyParts[len(childKeyParts)-1] == part {
				// Found the right child copy it.
				childCopy := *child
				node.Nodes[i] = &childCopy
				node = &childCopy
				continue partLoop
			}
		}

		// If we get here, the child wasn't known, create it.
		child := client.Node{
			Key: node.Key + "/" + part,
			Dir: true,  // We'll unset this again below if this is the leaf.
			CreatedIndex: e.nextIndex(),
			ModifiedIndex: e.nextIndex(),
		}
		node.Nodes = append(node.Nodes, &child)
		node = &child
	}
	node.Dir = false
	node.Value = value

	// Syncer only uses Action and Node fields:
	delta := &client.Response{
		Action: "set",
		Node: node,
	}

	// Record the new state and delta.
	e.states = append(e.states, node)
	e.deltaEvents = append(e.deltaEvents, delta)

	// Wake up anyone who was waiting for it.
	e.cond.Broadcast()
}


func (e *mockEtcd) Get(ctx context.Context, key string, opts *client.GetOptions) (*client.Response, error) {
	e.cond.L.Lock()
	defer e.cond.L.Unlock()

	parts := strings.Split(key, '/')
	node := e.states[e.currentIndex()]
	partLoop: for _, part := range parts {
		if part == "" {
			continue
		}
		for _, child := range node.Nodes {
			if child.Key == node.Key + "/" + part {
				node = child
				continue partLoop
			}
		}
		// If we get here, the key doesn't exist.
		return nil, &client.ClusterError{
			Errors: []error{client.Error{Code:client.ErrorCodeKeyNotFound}},
		}
	}

	if !opts.Recursive {
		// Non-recursive get, scrub the node of any children.
		nodeCopy := *node
		nodeCopy.Nodes = nil
		node = &nodeCopy
	}
	return &client.Response{
		Action: "get",
		Node: node,
		Index: e.currentIndex(),
		ClusterID: e.clusterID,
	}, nil
}

func (e *mockEtcd) Watcher(key string, opts *client.WatcherOptions) client.Watcher {
	e.cond.L.Lock()
	defer e.cond.L.Unlock()

	// Normalise the key to start and end with a slash.
	key = "/" + strings.Trim(key, "/") + "/"

	afterIndex := e.currentIndex()
		if opts.AfterIndex != 0 {
			afterIndex = opts.AfterIndex
		}
	if !opts.Recursive {
		panic("Only support recursive watches")
	}
	return &mockEtcdWatcher{
		etcd: e,
		key: key,
		afterIndex: afterIndex,
	}
}

type mockEtcdWatcher struct {
	etcd *mockEtcd
	key string
	afterIndex uint64
}

func (w *mockEtcdWatcher) Next(context.Context) (*client.Response, error) {
	w.etcd.cond.L.Lock()
	defer w.etcd.cond.L.Unlock()

	for {
		// Block until the datastore catches up with the event that we're looking for.
		for w.etcd.currentIndex() <= w.afterIndex {
			w.etcd.cond.Wait()
		}
		// Search all the new events to see if any of them match the prefix we're
		// watching for.
		for i := w.afterIndex+1; i<=w.etcd.currentIndex(); i++ {
			event := w.etcd.deltaEvents[i]
			if strings.HasPrefix(event.Node.Key, w.key) {
				// Found an event, update the index so we'll look for the next
				// event next time, then return it.
				w.afterIndex = i
				return event, nil
			}
		}
	}
}
