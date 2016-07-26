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

package labels

import (
	"github.com/golang/glog"
	"github.com/tigera/libcalico-go/datastructures/multidict"
	"github.com/tigera/libcalico-go/lib/selector"
)

type LabelInheritanceIndex interface {
	UpdateSelector(id interface{}, sel selector.Selector)
	DeleteSelector(id interface{})
	UpdateLabels(id interface{}, labels map[string]string, parents []string)
	DeleteLabels(id interface{})
	UpdateParentLabels(id string, labels map[string]string)
	DeleteParentLabels(id string)
}

type labelInheritanceIndex struct {
	index             Index
	labelsByItemID    map[interface{}]map[string]string
	labelsByParentID  map[interface{}]map[string]string
	parentIDsByItemID map[interface{}][]string
	itemIDsByParentID multidict.IfaceToIface
	dirtyItemIDs      map[interface{}]bool
}

func NewInheritanceIndex(onMatchStarted, onMatchStopped MatchCallback) LabelInheritanceIndex {
	index := NewIndex(onMatchStarted, onMatchStopped)
	inheritIDx := labelInheritanceIndex{
		index:             index,
		labelsByItemID:    make(map[interface{}]map[string]string),
		labelsByParentID:  make(map[interface{}]map[string]string),
		parentIDsByItemID: make(map[interface{}][]string),
		itemIDsByParentID: multidict.NewIfaceToIface(),
		dirtyItemIDs:      make(map[interface{}]bool),
	}
	return &inheritIDx
}

func (idx *labelInheritanceIndex) UpdateSelector(id interface{}, sel selector.Selector) {
	idx.index.UpdateSelector(id, sel)
}

func (idx *labelInheritanceIndex) DeleteSelector(id interface{}) {
	idx.index.DeleteSelector(id)
}

func (idx *labelInheritanceIndex) UpdateLabels(id interface{}, labels map[string]string, parents []string) {
	glog.V(3).Info("Inherit index updating labels for ", id)
	glog.V(4).Info("Num dirty items ", len(idx.dirtyItemIDs), " items")
	idx.labelsByItemID[id] = labels
	idx.onItemParentsUpdate(id, parents)
	idx.dirtyItemIDs[id] = true
	idx.flushUpdates()
	glog.V(4).Info("Num ending dirty items ", len(idx.dirtyItemIDs), " items")
}

func (idx *labelInheritanceIndex) DeleteLabels(id interface{}) {
	glog.V(3).Info("Inherit index deleting labels for ", id)
	delete(idx.labelsByItemID, id)
	idx.onItemParentsUpdate(id, []string{})
	idx.dirtyItemIDs[id] = true
	idx.flushUpdates()
}

func (idx *labelInheritanceIndex) onItemParentsUpdate(id interface{}, parents []string) {
	oldParents := idx.parentIDsByItemID[id]
	for _, parent := range oldParents {
		idx.itemIDsByParentID.Discard(parent, id)
	}
	if len(parents) > 0 {
		idx.parentIDsByItemID[id] = parents
	} else {
		delete(idx.parentIDsByItemID, id)
	}
	for _, parent := range parents {
		idx.itemIDsByParentID.Put(parent, id)
	}
}

func (idx *labelInheritanceIndex) UpdateParentLabels(parentID string, labels map[string]string) {
	idx.labelsByParentID[parentID] = labels
	idx.flushChildren(parentID)
}

func (idx *labelInheritanceIndex) DeleteParentLabels(parentID string) {
	delete(idx.labelsByParentID, parentID)
	idx.flushChildren(parentID)
}

func (idx *labelInheritanceIndex) flushChildren(parentID interface{}) {
	idx.itemIDsByParentID.Iter(parentID, func(itemID interface{}) {
		glog.V(4).Info("Marking child ", itemID, " dirty")
		idx.dirtyItemIDs[itemID] = true
	})
	idx.flushUpdates()
}

func (idx *labelInheritanceIndex) flushUpdates() {
	for itemID, _ := range idx.dirtyItemIDs {
		glog.V(4).Infof("Flushing %#v", itemID)
		itemLabels, ok := idx.labelsByItemID[itemID]
		if !ok {
			// Item deleted.
			glog.V(4).Infof("Flushing delete of item %v", itemID)
			idx.index.DeleteLabels(itemID)
		} else {
			// Item updated/created, re-evaluate labels.
			glog.V(4).Infof("Flushing update of item %v", itemID)
			combinedLabels := make(map[string]string)
			parentIDs := idx.parentIDsByItemID[itemID]
			for _, parentID := range parentIDs {
				parentLabels := idx.labelsByParentID[parentID]
				for k, v := range parentLabels {
					combinedLabels[k] = v
				}
			}
			for k, v := range itemLabels {
				combinedLabels[k] = v
			}
			idx.index.UpdateLabels(itemID, combinedLabels)
		}
	}
	idx.dirtyItemIDs = make(map[interface{}]bool)
}

var _ LabelInheritanceIndex = (*labelInheritanceIndex)(nil)
