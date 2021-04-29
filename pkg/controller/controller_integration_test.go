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

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllertest"
	"sigs.k8s.io/controller-runtime/pkg/controller/testdata/foo"
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
		f := &foo.Foo{}
		gvk := f.GroupVersionKind()
		options := manager.Options{}
		//s.AddKnownTypeWithName(gvk, f)
		//fl := &foo.FooList{}
		//s.AddKnownTypes(gvk.GroupVersion(), fl)
		foo.AddToScheme(s)
		options.Scheme = s
		crdPath := filepath.Join(".", "testadata", "crds", "foocrd.yaml")
		crdOpts := envtest.CRDInstallOptions{
			Paths: []string{crdPath},
		}
		_ = crdOpts

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
			ObjectMeta: metav1.ObjectMeta{Name: "test-foo"},
		}

		expectedReconcileRequest := reconcile.Request{NamespacedName: types.NamespacedName{
			Namespace: "default",
			Name:      "test-foo",
		}}
		_ = expectedReconcileRequest

		By("Creating the rest client")
		config := *cfg
		gv := gvk.GroupVersion()
		config.ContentConfig.GroupVersion = &gv
		config.NegotiatedSerializer = scheme.Codecs.WithoutConversion()
		client, err := rest.RESTClientFor(&config)
		Expect(err).NotTo(HaveOccurred())

		By("Failing to create a foo object if the crd isn't installed")
		result := foo.Foo{}
		err = client.Post().Namespace("default").Resource("foos").Body(testFoo).Do(ctx).Into(&result)
		Expect(err).To(HaveOccurred())

		By("Installing the CRD")
		fmt.Println("test installing crd")
		crds, err := envtest.InstallCRDs(cfg, crdOpts)
		fmt.Println("test crd installed")
		Expect(err).NotTo(HaveOccurred())
		Expect(len(crds)).To(Equal(1))

		By("Invoking Reconcile for foo Create")

		By("Uninstalling the CRD")
		//err = envtest.UninstallCRDs(cfg, crdOpts)
		//Expect(err).NotTo(HaveOccurred())

		By("Failing get foo object if the crd isn't installed")

		By("Reinstalling the CRD")

		By("Invoking Reconcile for foo Create")

		By("Uninstalling the CRD")

		close(done)

	}, 20)

})

func truePtr() *bool {
	t := true
	return &t
}
