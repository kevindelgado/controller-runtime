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
			fmt.Println("run the test")
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
			//for _, cb := range callbacks {
			//	cb(m)
			//}

			// create conditional runnable, add it to the manager
			var conditionalOn runtime.Object
			conditionalOn = &foo.Foo{}
			runCh := make(chan int)
			cache := m.GetCache()
			ctrl, err := controller.NewUnmanaged("testController", m, controller.Options{
				Reconciler: rec,
			})
			Expect(err).NotTo(HaveOccurred())
			//var internalCtrl *intctl.Controller
			internalCtrl, ok := ctrl.(*intctl.Controller)
			Expect(ok).To(BeTrue())
			stopCtrl := &fakeStoppableController{
				controller: internalCtrl,
				runCh:      runCh,
			}
			condCtrl := &controller.ConditionalController{
				Controller:      stopCtrl,
				Cache:           cache,
				ConditionalOn:   conditionalOn,
				DiscoveryClient: discovery.NewDiscoveryClientForConfigOrDie(cfg),
				Scheme:          m.GetScheme(),
				WaitTime:        5 * time.Millisecond,
			}
			Expect(m.Add(condCtrl)).NotTo(HaveOccurred())

			// start the manager in a separate go routine
			mgrStop := make(chan struct{})
			go func() {
				defer GinkgoRecover()
				Expect(m.Start(mgrStop)).NotTo(HaveOccurred())
				close(runCh)
			}()

			// run the test go routine to iterate through the situations where
			// the CRD is
			// 1) not installed
			// 2) installed
			// 3) uninstalled
			// 4) reinstalled
			// 5) uninstalled for test cleanup
			testLoopDone := make(chan struct{})
			fakeCtrl, ok := condCtrl.Controller.(*fakeStoppableController)
			Expect(ok).To(BeTrue())
			go func() {
				defer GinkgoRecover()
				//<-m.(*controllerManager).elected
				for i := 0; i < 5; i++ {
					if i%2 == 1 {
						// install CRD
						crds, err := envtest.InstallCRDs(cfg, crdOpts)
						Expect(err).NotTo(HaveOccurred())
						Expect(len(crds)).To(Equal(1))
					} else if i > 0 {
						// uninstall CRD
						err = envtest.UninstallCRDs(cfg, crdOpts)
						Expect(err).NotTo(HaveOccurred())
					}
					select {
					case <-runCh:
						// CRD is installed
						//Expect(i % 2).To(Equal(1))
					case <-time.After(50 * time.Millisecond):
						// CRD is NOT installed
						//Expect(i % 2).To(Equal(0))
						fakeCtrl.noStartCount++
					}
				}
				close(mgrStop)
				close(testLoopDone)
			}()
			<-testLoopDone
			fmt.Println("check expectations")
			fmt.Printf("fakeCtrl.startCount = %+v\n", fakeCtrl.startCount)
			fmt.Printf("fakeCtrl.noStartCount = %+v\n", fakeCtrl.noStartCount)
			Expect(fakeCtrl.startCount).To(Equal(2))
			Expect(fakeCtrl.noStartCount).To(Equal(3))
			fmt.Println("done the test!")
			close(done)
		})

	})
})

type fakeStoppableController struct {
	controller controller.StoppableController
	//conditionalOn runtime.Object
	startCount   int
	noStartCount int
	runCh        chan int
}

//Controller
func (f *fakeStoppableController) ResetStart() {
	f.controller.ResetStart()
}

func (f *fakeStoppableController) SaveWatches() {
	f.controller.SaveWatches()
}

func (f *fakeStoppableController) Start(s <-chan struct{}) error {
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
