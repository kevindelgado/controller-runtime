package foo

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/scheme"
)

type FooSpec struct{}
type FooStatus struct{}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

type Foo struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   FooSpec   `json:"spec,omitempty"`
	Status FooStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true
type FooList struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Items             []Foo `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Foo{}, &FooList{})
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

//func (in *FooList) DeepCopy() *FooList {
//	if in == nil {
//		return nil
//	}
//	out := new(FooList)
//	in.DeepCopyInto(out)
//	return out
//}

func (in *FooList) DeepCopyObject() runtime.Object {
	return nil
}

var (
	GroupVersion  = schema.GroupVersion{Group: "bar.example.com", Version: "v1"}
	SchemeBuilder = &scheme.Builder{GroupVersion: GroupVersion}
	AddToScheme   = SchemeBuilder.AddToScheme
)
