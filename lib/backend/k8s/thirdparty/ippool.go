package thirdparty

import (
	"encoding/json"

	"k8s.io/client-go/pkg/api"
	"k8s.io/client-go/pkg/api/meta"
	"k8s.io/client-go/pkg/api/unversioned"
	"k8s.io/client-go/pkg/runtime/schema"
)

type IpPoolSpec struct {
	Value string `json:"value"`
}

type IpPool struct {
	unversioned.TypeMeta `json:",inline"`
	Metadata             api.ObjectMeta `json:"metadata"`

	Spec IpPoolSpec `json:"spec"`
}

type IpPoolList struct {
	unversioned.TypeMeta `json:",inline"`
	Metadata             unversioned.ListMeta `json:"metadata"`

	Items []IpPool `json:"items"`
}

// Required to satisfy Object interface
func (e *IpPool) GetObjectKind() schema.ObjectKind {
	return &e.TypeMeta
}

// Required to satisfy ObjectMetaAccessor interface
func (e *IpPool) GetObjectMeta() meta.Object {
	return &e.Metadata
}

// Required to satisfy Object interface
func (el *IpPoolList) GetObjectKind() schema.ObjectKind {
	return &el.TypeMeta
}

// Required to satisfy ListMetaAccessor interface
func (el *IpPoolList) GetListMeta() unversioned.List {
	return &el.Metadata
}

// The code below is used only to work around a known problem with third-party
// resources and ugorji. If/when these issues are resolved, the code below
// should no longer be required.

type IpPoolListCopy IpPoolList
type IpPoolCopy IpPool

func (g *IpPool) UnmarshalJSON(data []byte) error {
	tmp := IpPoolCopy{}
	err := json.Unmarshal(data, &tmp)
	if err != nil {
		return err
	}
	tmp2 := IpPool(tmp)
	*g = tmp2
	return nil
}

func (l *IpPoolList) UnmarshalJSON(data []byte) error {
	tmp := IpPoolListCopy{}
	err := json.Unmarshal(data, &tmp)
	if err != nil {
		return err
	}
	tmp2 := IpPoolList(tmp)
	*l = tmp2
	return nil
}
