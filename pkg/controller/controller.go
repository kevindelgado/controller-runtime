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
	"fmt"
	"time"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/internal/controller"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/ratelimiter"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

// Options are the arguments for creating a new Controller
type Options struct {
	// MaxConcurrentReconciles is the maximum number of concurrent Reconciles which can be run. Defaults to 1.
	MaxConcurrentReconciles int

	// Reconciler reconciles an object
	Reconciler reconcile.Reconciler

	// RateLimiter is used to limit how frequently requests may be queued.
	// Defaults to MaxOfRateLimiter which has both overall and per-item rate limiting.
	// The overall is a token bucket and the per-item is exponential.
	RateLimiter ratelimiter.RateLimiter

	// Log is the logger used for this controller.
	Log logr.Logger

	// Conditionally is a flag set during controller setup that indicates it should conditionally wait on the
	// ForObject to appear in the discovery doc before starting the controller. If set,
	// it will set ConditionalObject to the ForObject being reconciled for in the controller builder.
	//Conditionally bool

	// ConditionalObject is the Object that the manager should wait on to appear
	// in the discovery document to indicate to begin running the controller
	// (and should stop the informer for this object if it disappears from the discovery doc).
	ConditionalOn *runtime.Object

	// ConditionalWaitTime is the frequency at which the controller manager should check
	// the discovery doc for the existence of the CondtionalOn object.
	ConditionalWaitTime time.Duration
}

// Controller implements a Kubernetes API.  A Controller manages a work queue fed reconcile.Requests
// from source.Sources.  Work is performed through the reconcile.Reconciler for each enqueued item.
// Work typically is reads and writes Kubernetes objects to make the system state match the state specified
// in the object Spec.
type Controller interface {
	// Reconciler is called to reconcile an object by Namespace/Name
	reconcile.Reconciler

	// Watch takes events provided by a Source and uses the EventHandler to
	// enqueue reconcile.Requests in response to the events.
	//
	// Watch may be provided one or more Predicates to filter events before
	// they are given to the EventHandler.  Events will be passed to the
	// EventHandler if all provided Predicates evaluate to true.
	Watch(src source.Source, eventhandler handler.EventHandler, predicates ...predicate.Predicate) error

	// Start starts the controller.  Start blocks until stop is closed or a
	// controller has an error starting.
	Start(stop <-chan struct{}) error
}

// New returns a new Controller registered with the Manager.  The Manager will ensure that shared Caches have
// been synced before the Controller is Started.
func New(name string, mgr manager.Manager, options Options) (Controller, error) {
	c, err := NewUnmanaged(name, mgr, options)
	if err != nil {
		return nil, err
	}

	// Add the controller as a Manager components
	return c, mgr.Add(c)
}

// NewUnmanaged returns a new controller without adding it to the manager. The
// caller is responsible for starting the returned controller.
func NewUnmanaged(name string, mgr manager.Manager, options Options) (Controller, error) {
	if options.Reconciler == nil {
		return nil, fmt.Errorf("must specify Reconciler")
	}

	if len(name) == 0 {
		return nil, fmt.Errorf("must specify Name for Controller")
	}

	if options.MaxConcurrentReconciles <= 0 {
		options.MaxConcurrentReconciles = 1
	}

	if options.RateLimiter == nil {
		options.RateLimiter = workqueue.DefaultControllerRateLimiter()
	}

	if options.Log == nil {
		options.Log = mgr.GetLogger()
	}

	// Inject dependencies into Reconciler
	if err := mgr.SetFields(options.Reconciler); err != nil {
		return nil, err
	}

	// Create controller with dependencies set
	return &controller.Controller{
		Do: options.Reconciler,
		MakeQueue: func() workqueue.RateLimitingInterface {
			return workqueue.NewNamedRateLimitingQueue(options.RateLimiter, name)
		},
		MaxConcurrentReconciles: options.MaxConcurrentReconciles,
		SetFields:               mgr.SetFields,
		Name:                    name,
		ConditionalOn:           options.ConditionalOn,
		ConditionalWaitTime:     options.ConditionalWaitTime,
		Log:                     options.Log.WithName("controller").WithValues("controller", name),
	}, nil
}
