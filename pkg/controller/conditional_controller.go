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
	"time"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/discovery"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

// TODO: Comment everything
// ConditionalController is a helper that wraps a controller that
// can be stopped and restarted (StoppableController).
//
// Unique to the ConditionalController is that only runs the underlying controller
// when the object that controller watches (ConditionalOn), is installed in the cluster.
//
// Otherwise it will wait and periodically (WaitTime) check the discovery doc for
// existence of the ConditionalOn, starting, stopping, and restarting the controller
// as necessary based on the presence/absence of ConditionalOn.
type ConditionalController struct {
	// Controller is the underlying controller that contains the Start()
	// to be ran when ConditionalOn exists in the cluster.
	Controller StoppableController

	// Cache is the manager's cache that must have the ConditionalOn object
	// removed upon stopping the Controller.
	Cache cache.Cache

	// ConditionalOn is the object being controlled by the Controller
	// and for whose existence in the cluster/discover doc is required in order
	// for the Controller to be running.
	ConditionalOn runtime.Object

	// DiscoveryClient is used to query the discover doc for the existence
	// of the ConditionalOn in the cluster.
	DiscoveryClient *discovery.DiscoveryClient

	// Scheme helps convert between gvk and object.
	Scheme *runtime.Scheme

	// WaitTime is how long to wait before rechecking the discovery doc.
	WaitTime time.Duration
}

// StoppableController is a wrapper around Controller providing extra methods
// that allow for running the controller multiple times.
type StoppableController interface {
	Controller

	// ResetStart sets Started to false to enable running Start on the controller again.
	ResetStart()

	// SaveWatches indicates that watches should not be cleared when the controller is stopped.
	SaveWatches()
}

// Reconcile implements the Controller interface.
func (c *ConditionalController) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	return c.Controller.Reconcile(ctx, req)
}

// Watch implements the Controller interface.
func (c *ConditionalController) Watch(src source.Source, eventhandler handler.EventHandler, predicates ...predicate.Predicate) error {
	return c.Controller.Watch(src, eventhandler, predicates...)
}

// Start condtionally runs the underlying controller based on the existence of the ConditionalOn in the cluster.
// In it's absence it waits for WaitTime before checking the discovery doc again.
func (c *ConditionalController) Start(ctx context.Context) error {
	prevInstalled := false
	curInstalled := false
	errChan := make(chan error)
	var mergeCtx context.Context
	var cancel context.CancelFunc
	//var presentStop chan struct{}
	for {
		select {
		case err := <-errChan:
			return err
		case <-ctx.Done():
			return nil
		case <-time.After(c.WaitTime):
			gvk, err := apiutil.GVKForObject(c.ConditionalOn, c.Scheme)
			if err != nil {
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
			if !prevInstalled && curInstalled {
				// Going from not installed -> installed.
				// Start the runnable.
				//presentStop = make(chan struct{})
				//mergedStop := mergeChan(presentStop, stop)
				mergeCtx, cancel = context.WithCancel(ctx)
				prevInstalled = true
				go func() {
					if err := c.Controller.Start(mergeCtx); err != nil {
						errChan <- err
					}
				}()
			} else if prevInstalled && !curInstalled {
				// Going from installed -> not installed.
				// Stop the runnable and remove the obj's informer from the cache.
				// It's safe to remove the obj's informer because anything that is
				// using it's informer will no longer work because the obj has been
				// uninstalled from the cluster.
				c.Controller.ResetStart()
				c.Controller.SaveWatches()
				//close(presentStop)
				cancel()
				// if we don't sleep, the cache will remove before the cancel propagates to remove the event handler
				// TODO: Fix
				time.Sleep(time.Second)
				if err := c.Cache.Remove(ctx, c.ConditionalOn); err != nil {
					return err
				}
				prevInstalled = false
			}
		}
	}

}

// mergeChan return channel fires when either channel a or channel b is fired.
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
