package foo

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type FooSpec struct{}
type FooStatus struct{}

type Foo struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   FooSpec   `json:"spec,omitempty"`
	Status FooStatus `json:"status,omitempty"`
}

func (f *Foo) DeepCopyObject() runtime.Object {
	return nil
}

func (f *Foo) SetGroupVersionKind(kind schema.GroupVersionKind) {}
func (f *Foo) GroupVersionKind() schema.GroupVersionKind {
	return schema.GroupVersionKind{
		Group:   "bar.example.com",
		Version: "v1",
		Kind:    "Foo",
	}
}

func (f *Foo) GetObjectKind() schema.ObjectKind {
	return f

}
