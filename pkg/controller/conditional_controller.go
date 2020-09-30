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

package controller

import (
	"context"
	"fmt"
	"time"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/discovery"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	intctrl "sigs.k8s.io/controller-runtime/pkg/internal/controller"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

type ConditionalController struct {
	Controller      intctrl.Controller
	Cache           *cache.Cache
	ConditionalOn   runtime.Object
	DiscoveryClient *discovery.DiscoveryClient
	Scheme          *runtime.Scheme
	WaitTime        time.Duration
}

func (c *ConditionalController) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	return c.Controller.Reconcile(ctx, req)
}

func (c *ConditionalController) Watch(src source.Source, eventhandler handler.EventHandler, predicates ...predicate.Predicate) error {
	return c.Controller.Watch(src, eventhandler, predicates...)
}

func (c *ConditionalController) Start(stop <-chan struct{}) error {
	fmt.Println("Start! this is it boys")
	prevInstalled := false
	curInstalled := false
	errChan := make(chan error)
	var presentStop chan struct{}
	for {
		//fmt.Println("looping")
		//c.mu.Lock()
		select {
		case err := <-errChan:
			return err
		case <-stop:
			//fmt.Println("stopping")
			return nil
		case <-time.After(c.WaitTime):
			//fmt.Println("wait done")
			gvk, err := apiutil.GVKForObject(c.ConditionalOn, c.Scheme)
			if err != nil {
				//log.Error(err, "could not resolve gvk for conditional runnable obj")
				//c.mu.Unlock()
				break
			}
			resources, err := c.DiscoveryClient.ServerResourcesForGroupVersion(gvk.GroupVersion().String())
			if err != nil {
				curInstalled = false
			} else {
				curInstalled = false
				for _, res := range resources.APIResources {
					if res.Kind == gvk.Kind {
						curInstalled = true
					}
				}
			}
			//fmt.Printf("prevInstalled, curInstalled = %+v, %+v\n", prevInstalled, curInstalled)
			if !prevInstalled && curInstalled {
				//fmt.Println("Installed!!!")
				// Going from not installed -> installed.
				// Start the runnable.
				presentStop = make(chan struct{})
				mergedStop := mergeChan(presentStop, stop)
				go func() {
					if err := c.Controller.Start(mergedStop); err != nil {
						errChan <- err
					}
				}()
				prevInstalled = true
			} else if prevInstalled && !curInstalled {
				// Going from installed -> not installed.
				// Stop the runnable and remove the obj's informer from the cache.
				// It's safe to remove the obj's informer because anything that is
				// using it's informer will no longer work because the obj has been
				// uninstalled from the cluster.
				//fmt.Println("UNINSTALLED")
				close(presentStop)
				//if err := c.Controller.Cache.Remove(c.ConditionalOn); err != nil {
				if err := (*c.Cache).Remove(c.ConditionalOn); err != nil {
					//if err := c.kind.GetCache().Remove(c.ConditionalOn); err != nil {
					//fmt.Printf("CACHE err = %+v\n", err)
					return err
				}
				c.Controller.Started = false
				prevInstalled = false
			}
		}
		//c.mu.Unlock()
		//fmt.Println("finished loop")
	}

}

func mergeChan(a, b <-chan struct{}) chan struct{} {
	out := make(chan struct{})
	go func() {
		defer close(out)
		select {
		case <-a:
		case <-b:
		}
	}()
	return out
}
