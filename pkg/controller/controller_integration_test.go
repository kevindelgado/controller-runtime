/*
Copyright 2018 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controller_test

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllertest"
	foo "sigs.k8s.io/controller-runtime/pkg/controller/testdata/foo/v1"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

var _ = Describe("controller", func() {
	var reconciled chan reconcile.Request
	ctx := context.Background()

	BeforeEach(func() {
		reconciled = make(chan reconcile.Request)
		Expect(cfg).NotTo(BeNil())
	})

	Describe("controller", func() {
		// TODO(directxman12): write a whole suite of controller-client interaction tests

		It("should reconcile", func(done Done) {
			By("Creating the Manager")
			cm, err := manager.New(cfg, manager.Options{})
			Expect(err).NotTo(HaveOccurred())

			By("Creating the Controller")
			instance, err := controller.New("foo-controller", cm, controller.Options{
				Reconciler: reconcile.Func(
					func(_ context.Context, request reconcile.Request) (reconcile.Result, error) {
						reconciled <- request
						return reconcile.Result{}, nil
					}),
			})
			Expect(err).NotTo(HaveOccurred())

			By("Watching Resources")
			err = instance.Watch(&source.Kind{Type: &appsv1.ReplicaSet{}}, &handler.EnqueueRequestForOwner{
				OwnerType: &appsv1.Deployment{},
			})
			Expect(err).NotTo(HaveOccurred())

			err = instance.Watch(&source.Kind{Type: &appsv1.Deployment{}}, &handler.EnqueueRequestForObject{})
			Expect(err).NotTo(HaveOccurred())

			err = cm.GetClient().Get(ctx, types.NamespacedName{Name: "foo"}, &corev1.Namespace{})
			Expect(err).To(Equal(&cache.ErrCacheNotStarted{}))
			err = cm.GetClient().List(ctx, &corev1.NamespaceList{})
			Expect(err).To(Equal(&cache.ErrCacheNotStarted{}))

			By("Starting the Manager")
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			go func() {
				defer GinkgoRecover()
				Expect(cm.Start(ctx)).NotTo(HaveOccurred())
			}()

			deployment := &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{Name: "deployment-name"},
				Spec: appsv1.DeploymentSpec{
					Selector: &metav1.LabelSelector{
						MatchLabels: map[string]string{"foo": "bar"},
					},
					Template: corev1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"foo": "bar"}},
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{
								{
									Name:  "nginx",
									Image: "nginx",
									SecurityContext: &corev1.SecurityContext{
										Privileged: truePtr(),
									},
								},
							},
						},
					},
				},
			}
			expectedReconcileRequest := reconcile.Request{NamespacedName: types.NamespacedName{
				Namespace: "default",
				Name:      "deployment-name",
			}}

			By("Invoking Reconciling for Create")
			deployment, err = clientset.AppsV1().Deployments("default").Create(ctx, deployment, metav1.CreateOptions{})
			Expect(err).NotTo(HaveOccurred())
			Expect(<-reconciled).To(Equal(expectedReconcileRequest))

			By("Invoking Reconciling for Update")
			newDeployment := deployment.DeepCopy()
			newDeployment.Labels = map[string]string{"foo": "bar"}
			_, err = clientset.AppsV1().Deployments("default").Update(ctx, newDeployment, metav1.UpdateOptions{})
			Expect(err).NotTo(HaveOccurred())
			Expect(<-reconciled).To(Equal(expectedReconcileRequest))

			By("Invoking Reconciling for an OwnedObject when it is created")
			replicaset := &appsv1.ReplicaSet{
				ObjectMeta: metav1.ObjectMeta{
					Name: "rs-name",
					OwnerReferences: []metav1.OwnerReference{
						*metav1.NewControllerRef(deployment, schema.GroupVersionKind{
							Group:   "apps",
							Version: "v1",
							Kind:    "Deployment",
						}),
					},
				},
				Spec: appsv1.ReplicaSetSpec{
					Selector: &metav1.LabelSelector{
						MatchLabels: map[string]string{"foo": "bar"},
					},
					Template: deployment.Spec.Template,
				},
			}
			replicaset, err = clientset.AppsV1().ReplicaSets("default").Create(ctx, replicaset, metav1.CreateOptions{})
			Expect(err).NotTo(HaveOccurred())
			Expect(<-reconciled).To(Equal(expectedReconcileRequest))

			By("Invoking Reconciling for an OwnedObject when it is updated")
			newReplicaset := replicaset.DeepCopy()
			newReplicaset.Labels = map[string]string{"foo": "bar"}
			_, err = clientset.AppsV1().ReplicaSets("default").Update(ctx, newReplicaset, metav1.UpdateOptions{})
			Expect(err).NotTo(HaveOccurred())
			Expect(<-reconciled).To(Equal(expectedReconcileRequest))

			By("Invoking Reconciling for an OwnedObject when it is deleted")
			err = clientset.AppsV1().ReplicaSets("default").Delete(ctx, replicaset.Name, metav1.DeleteOptions{})
			Expect(err).NotTo(HaveOccurred())
			Expect(<-reconciled).To(Equal(expectedReconcileRequest))

			By("Invoking Reconciling for Delete")
			err = clientset.AppsV1().Deployments("default").
				Delete(ctx, "deployment-name", metav1.DeleteOptions{})
			Expect(err).NotTo(HaveOccurred())
			Expect(<-reconciled).To(Equal(expectedReconcileRequest))

			By("Listing a type with a slice of pointers as items field")
			err = cm.GetClient().
				List(context.Background(), &controllertest.UnconventionalListTypeList{})
			Expect(err).NotTo(HaveOccurred())

			close(done)
		}, 5)
	})

	FIt("should reconcile when the CRD is installed, uninstalled, reinstalled", func(done Done) {
		By("Initializing the scheme and crd")
		s := runtime.NewScheme()
		//s.AddKnownTypeWithName(gvk, f)
		//fl := &foo.FooList{}
		//s.AddKnownTypes(gvk.GroupVersion(), fl)
		err := v1beta1.AddToScheme(s)
		Expect(err).NotTo(HaveOccurred())
		err = apiextensionsv1.AddToScheme(s)
		Expect(err).NotTo(HaveOccurred())
		err = foo.AddToScheme(s)
		Expect(err).NotTo(HaveOccurred())
		options := manager.Options{Scheme: s}

		By("Creating the Manager")
		cm, err := manager.New(cfg, options)
		Expect(err).NotTo(HaveOccurred())

		By("Creating the Controller")
		instance, err := controller.New("foo-controller", cm, controller.Options{
			Reconciler: reconcile.Func(
				func(_ context.Context, request reconcile.Request) (reconcile.Result, error) {
					reconciled <- request
					return reconcile.Result{}, nil
				}),
		})
		Expect(err).NotTo(HaveOccurred())

		By("Watching foo CRD as sporadic kinds")
		f := &foo.Foo{}
		//gvk := f.GroupVersionKind()
		gvk := schema.GroupVersionKind{
			Group:   "bar.example.com",
			Version: "v1",
			Kind:    "Foo",
		}
		fmt.Printf("gvk = %+v\n", gvk)
		dc := clientset.Discovery()
		Expect(err).NotTo(HaveOccurred())
		existsInDiscovery := func() bool {
			resources, err := dc.ServerResourcesForGroupVersion(gvk.GroupVersion().String())
			if err != nil {
				fmt.Printf("NOT in discovery gvk = %+v\n", gvk)
				return false
			}
			for _, res := range resources.APIResources {
				if res.Kind == gvk.Kind {
					fmt.Printf("YES in discovery gvk = %+v\n", gvk)
					return true
				}
			}
			fmt.Printf("NOT in discovery kind = %+v\n", gvk)
			return false
		}
		err = instance.Watch(&source.SporadicKind{Kind: source.Kind{Type: f}, DiscoveryCheck: existsInDiscovery}, &handler.EnqueueRequestForObject{})
		Expect(err).NotTo(HaveOccurred())
		//err = cm.GetClient().Get(ctx, types.NamespacedName{Name: "foo"}, &corev1.Namespace{})
		//err = cm.GetClient().Get(context.TODO(), types.NamespacedName{Name: "foos.bar.example.com"}, &corev1.Namespace{})
		//Expect(err).To(Equal(&cache.ErrCacheNotStarted{}))
		//err = cm.GetClient().List(ctx, &corev1.NamespaceList{})
		//Expect(err).To(Equal(&cache.ErrCacheNotStarted{}))

		By("Starting the Manager")
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		go func() {
			defer GinkgoRecover()
			Expect(cm.Start(ctx)).NotTo(HaveOccurred())
		}()

		testFoo := &foo.Foo{
			TypeMeta:   metav1.TypeMeta{Kind: gvk.Kind, APIVersion: gvk.GroupVersion().String()},
			ObjectMeta: metav1.ObjectMeta{Name: "test-foo", Namespace: "default"},
		}

		expectedReconcileRequest := reconcile.Request{NamespacedName: types.NamespacedName{
			Namespace: "default",
			Name:      "test-foo",
		}}
		_ = expectedReconcileRequest

		By("Creating the client")
		//config := *cfg
		//gv := gvk.GroupVersion()
		//config.ContentConfig.GroupVersion = &gv
		//config.NegotiatedSerializer = scheme.Codecs.WithoutConversion()
		//client, err := rest.RESTClientFor(&config)
		// TODO: I don't think Mapper is actually necessary, recheck once working
		//c, err := client.New(cfg, client.Options{Scheme: s, Mapper: cm.GetRESTMapper()})
		c := cm.GetClient()
		Expect(err).NotTo(HaveOccurred())

		By("Failing to create a foo object if the crd isn't installed")
		//result := foo.Foo{}
		//err = client.Post().Namespace("default").Resource("foos").Body(testFoo).Do(ctx).Into(&result)
		err = c.Create(ctx, testFoo)
		Expect(err).To(HaveOccurred())

		// TODO: remove, see if necessary
		//time.Sleep(6 * time.Second)

		By("Installing the CRD")
		fmt.Println("test installing crd")

		crdPath := filepath.Join(".", "testdata", "foo", "foocrd.yaml")
		// TODO: fmt
		crdOpts := envtest.CRDInstallOptions{
			Paths:        []string{crdPath},
			MaxTime:      50 * time.Millisecond,
			PollInterval: 15 * time.Millisecond,
		}
		crds, err := envtest.InstallCRDs(cfg, crdOpts)
		fmt.Println("test crd installed")
		fmt.Printf("err = %+v\n", err)
		fmt.Printf("crds = %+v\n", crds[0])
		Expect(err).NotTo(HaveOccurred())
		Expect(len(crds)).To(Equal(1))

		By("Expecting to find the CRD")
		crdv1 := &apiextensionsv1.CustomResourceDefinition{}
		err = c.Get(context.TODO(), types.NamespacedName{Name: "foos.bar.example.com"}, crdv1)
		Expect(err).NotTo(HaveOccurred())
		Expect(crdv1.Spec.Names.Kind).To(Equal("Foo"))

		err = envtest.WaitForCRDs(cfg, []client.Object{
			&v1beta1.CustomResourceDefinition{
				Spec: v1beta1.CustomResourceDefinitionSpec{
					Group: "bar.example.com",
					Names: v1beta1.CustomResourceDefinitionNames{
						Kind:   "Foo",
						Plural: "foos",
					},
					Versions: []v1beta1.CustomResourceDefinitionVersion{
						{
							Name:    "v1",
							Storage: true,
							Served:  true,
						},
					}},
			},
		},
			crdOpts,
		//envtest.CRDInstallOptions{MaxTime: 50 * time.Millisecond, PollInterval: 15 * time.Millisecond},
		)
		Expect(err).NotTo(HaveOccurred())
		fmt.Println("crds done waiting")

		// sleep
		// TODO: remove once working if not needed
		//time.Sleep(6 * time.Second)

		By("Invoking Reconcile for foo Create")
		fmt.Println("post foo")
		//result = foo.Foo{}
		//err = client.Post().Namespace("default").Resource("foos").Body(testFoo).Do(ctx).Into(&result)
		err = c.Create(ctx, testFoo)
		fmt.Printf("err = %+v\n", err)
		Expect(err).NotTo(HaveOccurred())
		Expect(<-reconciled).To(Equal(expectedReconcileRequest))

		By("Uninstalling the CRD")
		errNotFound := errors.NewGenericServerResponse(404, "POST", schema.GroupResource{Group: "bar.example.com", Resource: "foos"}, "", "404 page not found", 0, true)
		err = envtest.UninstallCRDs(cfg, crdOpts)
		Expect(err).NotTo(HaveOccurred())
		// wait for discovery to not recognize the resource after uninstall
		wait.PollImmediate(15*time.Millisecond, 50*time.Millisecond, func() (bool, error) {
			if _, err := clientset.Discovery().ServerResourcesForGroupVersion(gvk.Group + "/" + gvk.Version); err != nil {
				if err.Error() == "the server could not find the requested resource" {
					return true, nil
				}
			}
			return false, nil
		})

		By("Failing create foo object if the crd isn't installed")
		err = c.Create(ctx, testFoo)
		// TODO: check the error is the correct one
		fmt.Printf("fail err = %+v\n", err)
		//Expect(err).To(HaveOccurred())
		//errNotFound := errors.NewNotFound(schema.GroupResource{Group: "bar.example.com", Resource: "foos"}, "")
		Expect(err).To(Equal(errNotFound))
		//Expect(err).NotTo(HaveOccurred())

		By("Reinstalling the CRD")
		crds, err = envtest.InstallCRDs(cfg, crdOpts)
		fmt.Println("test crd installed")
		fmt.Printf("err = %+v\n", err)
		fmt.Printf("crds = %+v\n", crds[0])
		Expect(err).NotTo(HaveOccurred())
		Expect(len(crds)).To(Equal(1))

		By("Expecting to find the CRD")
		crdv1 = &apiextensionsv1.CustomResourceDefinition{}
		err = c.Get(context.TODO(), types.NamespacedName{Name: "foos.bar.example.com"}, crdv1)
		Expect(err).NotTo(HaveOccurred())
		Expect(crdv1.Spec.Names.Kind).To(Equal("Foo"))

		err = envtest.WaitForCRDs(cfg, []client.Object{
			&v1beta1.CustomResourceDefinition{
				Spec: v1beta1.CustomResourceDefinitionSpec{
					Group: "bar.example.com",
					Names: v1beta1.CustomResourceDefinitionNames{
						Kind:   "Foo",
						Plural: "foos",
					},
					Versions: []v1beta1.CustomResourceDefinitionVersion{
						{
							Name:    "v1",
							Storage: true,
							Served:  true,
						},
					}},
			},
		},
			crdOpts,
		//envtest.CRDInstallOptions{MaxTime: 50 * time.Millisecond, PollInterval: 15 * time.Millisecond},
		)
		Expect(err).NotTo(HaveOccurred())
		fmt.Println("crds done waiting")

		By("Invoking Reconcile for foo Create")
		fmt.Println("post foo")
		//result = foo.Foo{}
		//err = client.Post().Namespace("default").Resource("foos").Body(testFoo).Do(ctx).Into(&result)
		// TODO: create a new testFoo?
		// clear the resourceVersion
		testFoo.ResourceVersion = ""
		err = c.Create(ctx, testFoo)
		fmt.Printf("err = %+v\n", err)
		Expect(err).NotTo(HaveOccurred())
		Expect(<-reconciled).To(Equal(expectedReconcileRequest))

		By("Uninstalling the CRD")
		err = envtest.UninstallCRDs(cfg, crdOpts)
		Expect(err).NotTo(HaveOccurred())
		// wait for discovery to not recognize the resource after uninstall
		wait.PollImmediate(15*time.Millisecond, 50*time.Millisecond, func() (bool, error) {
			if _, err := clientset.Discovery().ServerResourcesForGroupVersion(gvk.Group + "/" + gvk.Version); err != nil {
				if err.Error() == "the server could not find the requested resource" {
					return true, nil
				}
			}
			return false, nil
		})

		By("Failing create foo object if the crd isn't installed")
		err = c.Create(ctx, testFoo)
		Expect(err).To(Equal(errNotFound))

		fmt.Println("done")
		close(done)

		// TODO: make the test faster?
	}, 25)

})

func truePtr() *bool {
	t := true
	return &t
}
