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
	"path/filepath"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/discovery"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/testdata/foo"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	intctl "sigs.k8s.io/controller-runtime/pkg/internal/controller"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

var _ = Describe("controller.ConditionalController", func() {
	var stop chan struct{}

	rec := reconcile.Func(func(context.Context, reconcile.Request) (reconcile.Result, error) {
		return reconcile.Result{}, nil
	})
	BeforeEach(func() {
		stop = make(chan struct{})
	})

	AfterEach(func() {
		close(stop)
	})

	Describe("Start", func() {

		It("should run manager for conditional runnables when corresponding CRD is not installed, installed, uninstalled and reinstalled", func(done Done) {
			// initinalize scheme, crdOpts
			s := runtime.NewScheme()
			f := &foo.Foo{}
			options := manager.Options{}
			s.AddKnownTypeWithName(f.GroupVersionKind(), f)
			options.Scheme = s
			crdPath := filepath.Join(".", "testdata", "crds", "foocrd.yaml")
			crdOpts := envtest.CRDInstallOptions{
				Paths: []string{crdPath},
			}

			// create manager
			m, err := manager.New(cfg, options)
			Expect(err).NotTo(HaveOccurred())

			// create conditional controller, add it to the manager
			var conditionalOn runtime.Object
			conditionalOn = &foo.Foo{}
			runCh := make(chan int)
			cache := m.GetCache()
			ctrl, err := controller.NewUnmanaged("testController", m, controller.Options{
				Reconciler: rec,
			})
			Expect(err).NotTo(HaveOccurred())
			internalCtrl, ok := ctrl.(*intctl.Controller)
			Expect(ok).To(BeTrue())
			stopCtrl := &fakeStoppableController{
				controller: internalCtrl,
				runCh:      runCh,
			}

			dc := discovery.NewDiscoveryClientForConfigOrDie(cfg)
			condCtrl := &controller.ConditionalController{
				Controller:      stopCtrl,
				Cache:           cache,
				ConditionalOn:   conditionalOn,
				DiscoveryClient: dc,
				Scheme:          m.GetScheme(),
				WaitTime:        20 * time.Millisecond,
			}
			Expect(m.Add(condCtrl)).NotTo(HaveOccurred())

			//gvk, err := apiutil.GVKForObject(conditionalOn, m.GetScheme())
			//Expect(err).NotTo(HaveOccurred())
			//installed := true
			//for installed {
			//	resources, err := dc.ServerResourcesForGroupVersion(gvk.GroupVersion().String())
			//	Expect(err).NotTo(HaveOccurred())
			//	fmt.Printf("main gvk.GroupVersion().String() = %+v\n", gvk.GroupVersion().String())
			//	fmt.Printf("main gvk.Kind = %+v\n", gvk.Kind)
			//	for _, res := range resources.APIResources {
			//		if res.Kind == gvk.Kind {
			//			fmt.Println("already INSTALLED!")
			//			err = envtest.UninstallCRDs(cfg, crdOpts)
			//			Expect(err).NotTo(HaveOccurred())
			//			installed = true
			//			fmt.Println("Better be uninstalled")
			//			break

			//		}
			//	}

			//}

			// start the manager in a separate go routine
			mgrStop := make(chan struct{})
			testLoopStart := make(chan struct{})
			go func() {
				defer GinkgoRecover()
				close(testLoopStart)
				Expect(m.Start(mgrStop)).NotTo(HaveOccurred())
				close(runCh)
			}()

			// run the test go routine to iterate through the situations where
			// the CRD is
			// 0) not installed
			// 1) installed
			// 2) uninstalled
			// 3) reinstalled
			// 4) uninstalled for test cleanup
			testLoopDone := make(chan struct{})
			fakeCtrl, ok := condCtrl.Controller.(*fakeStoppableController)
			Expect(ok).To(BeTrue())
			go func() {
				defer GinkgoRecover()
				<-testLoopStart
				for i := 0; i < 5; i++ {
					if i%2 == 1 {
						// install CRD
						//fmt.Println("installing CRD")
						crds, err := envtest.InstallCRDs(cfg, crdOpts)
						Expect(err).NotTo(HaveOccurred())
						Expect(len(crds)).To(Equal(1))
					} else if i > 0 {
						// uninstall CRD
						//fmt.Println("uninstalling CRD")
						err = envtest.UninstallCRDs(cfg, crdOpts)
						Expect(err).NotTo(HaveOccurred())
					}
					select {
					case <-runCh:
						//fmt.Println("START bumped")
						//fmt.Printf("i = %+v\n", i)
						//Expect(i % 2).To(Equal(1))
						// CRD is installed
					case <-time.After(60 * time.Millisecond):
						//fmt.Println("NO start bumped")
						//fmt.Printf("i = %+v\n", i)
						//Expect(i % 2).To(Equal(0))
						// CRD is NOT installed
						fakeCtrl.noStartCount++
					}
				}
				close(mgrStop)
				close(testLoopDone)
			}()
			<-testLoopDone
			Expect(fakeCtrl.startCount).To(Equal(2))
			Expect(fakeCtrl.noStartCount).To(Equal(3))
			close(done)
		})

	})
})

type fakeStoppableController struct {
	controller   controller.StoppableController
	startCount   int
	noStartCount int
	runCh        chan int
}

// Methods for fakeStoppableController to conform to the StoppableController interface
func (f *fakeStoppableController) ResetStart() {
	f.controller.ResetStart()
}

func (f *fakeStoppableController) SaveWatches() {
	f.controller.SaveWatches()
}

func (f *fakeStoppableController) Start(s <-chan struct{}) error {
	//fmt.Println("bumping start")
	f.startCount++
	f.runCh <- 1
	return nil
}

func (f *fakeStoppableController) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	return f.controller.Reconcile(ctx, req)
}

func (f *fakeStoppableController) Watch(src source.Source, eventhandler handler.EventHandler, predicates ...predicate.Predicate) error {
	return f.controller.Watch(src, eventhandler, predicates...)
}
