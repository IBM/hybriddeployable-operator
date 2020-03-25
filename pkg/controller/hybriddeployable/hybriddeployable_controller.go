// Copyright 2019 The Kubernetes Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package hybriddeployable

import (
	"context"
	"reflect"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	dplv1alpha1 "github.com/IBM/multicloud-operators-deployable/pkg/apis/app/v1alpha1"
	placementv1alpha1 "github.com/IBM/multicloud-operators-placementrule/pkg/apis/app/v1alpha1"

	deployerv1alpha1 "github.com/IBM/deployer-operator/pkg/apis/app/v1alpha1"
	appv1alpha1 "github.com/IBM/hybriddeployable-operator/pkg/apis/app/v1alpha1"
	"github.com/IBM/hybriddeployable-operator/pkg/utils"
)

const (
	packageInfoLogLevel   = 3
	packageDetailLogLevel = 5
)

/**
* USER ACTION REQUIRED: This is a scaffold file intended for the user to modify with their own Controller
* business logic.  Delete these comments after modifying this file.*
 */

// Add creates a new HybridDeployable Controller and adds it to the Manager. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	reconciler := &ReconcileHybridDeployable{Client: mgr.GetClient()}

	err := reconciler.initRegistry(mgr.GetConfig())
	if err != nil {
		klog.Error("Failed to initialize hybrid deployable registry with error:", err)
		return nil
	}

	reconciler.eventRecorder, err = utils.NewEventRecorder(mgr.GetConfig(), mgr.GetScheme())
	if err != nil {
		klog.Error("Failed to initialize hybrid deployable registry with error:", err)
		return nil
	}

	return reconciler
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New("hybriddeployable-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to primary resource HybridDeployable
	err = c.Watch(
		&source.Kind{Type: &appv1alpha1.HybridDeployable{}},
		&handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	err = c.Watch(
		&source.Kind{
			Type: &appv1alpha1.HybridDeployable{}},
		&handler.EnqueueRequestsFromMapFunc{
			ToRequests: &hdplstatusMapper{mgr.GetClient()},
		},
		predicate.Funcs{
			UpdateFunc: func(e event.UpdateEvent) bool {
				newpr := e.ObjectNew.(*appv1alpha1.HybridDeployable)
				oldpr := e.ObjectOld.(*appv1alpha1.HybridDeployable)

				return !reflect.DeepEqual(oldpr.Status, newpr.Status)
			},
		},
	)
	if err != nil {
		return err
	}

	err = c.Watch(
		&source.Kind{
			Type: &deployerv1alpha1.DeployerSet{}},
		&handler.EnqueueRequestsFromMapFunc{
			ToRequests: &deployersetMapper{mgr.GetClient()},
		},
	)
	if err != nil {
		return err
	}

	err = c.Watch(
		&source.Kind{
			Type: &deployerv1alpha1.Deployer{}},
		&handler.EnqueueRequestsFromMapFunc{
			ToRequests: &deployerMapper{mgr.GetClient()},
		},
	)
	if err != nil {
		return err
	}

	err = c.Watch(
		&source.Kind{
			Type: &placementv1alpha1.PlacementRule{}},
		&handler.EnqueueRequestsFromMapFunc{
			ToRequests: &placementruleMapper{mgr.GetClient()},
		},
		predicate.Funcs{
			UpdateFunc: func(e event.UpdateEvent) bool {
				newpr := e.ObjectNew.(*placementv1alpha1.PlacementRule)
				oldpr := e.ObjectOld.(*placementv1alpha1.PlacementRule)

				return !reflect.DeepEqual(oldpr.Status, newpr.Status)
			},
		},
	)

	if err != nil {
		return err
	}

	err = c.Watch(
		&source.Kind{
			Type: &dplv1alpha1.Deployable{}},
		&handler.EnqueueRequestsFromMapFunc{
			ToRequests: &outputMapper{mgr.GetClient()},
		},
	)
	if err != nil {
		return err
	}

	err = c.Watch(
		&source.Kind{
			Type: &corev1.Endpoints{}},
		&handler.EnqueueRequestsFromMapFunc{
			ToRequests: &outputMapper{mgr.GetClient()},
		},
	)
	if err != nil {
		return err
	}

	err = c.Watch(
		&source.Kind{
			Type: &corev1.Secret{}},
		&handler.EnqueueRequestsFromMapFunc{
			ToRequests: &outputMapper{mgr.GetClient()},
		},
	)
	if err != nil {
		return err
	}

	err = c.Watch(
		&source.Kind{
			Type: &corev1.ConfigMap{}},
		&handler.EnqueueRequestsFromMapFunc{
			ToRequests: &outputMapper{mgr.GetClient()},
		},
	)
	if err != nil {
		return err
	}

	return nil
}

// blank assignment to verify that ReconcileHybridDeployable implements reconcile.Reconciler
var _ reconcile.Reconciler = &ReconcileHybridDeployable{}

type hdplstatusMapper struct {
	client.Client
}

func (mapper *hdplstatusMapper) Map(obj handler.MapObject) []reconcile.Request {
	var requests []reconcile.Request

	hdplList := &appv1alpha1.HybridDeployableList{}

	err := mapper.List(context.TODO(), hdplList, &client.ListOptions{})
	if err != nil {
		klog.Error("Failed to list hybrid deployables for deployerset with error:", err)
		return requests
	}

	depname := obj.Meta.GetName()
	depns := obj.Meta.GetNamespace()

	for _, hdpl := range hdplList.Items {
		if hdpl.Spec.Dependencies == nil {
			continue
		}

		for _, depref := range hdpl.Spec.Dependencies {
			if depref.Name != depname {
				continue
			}

			if depref.Namespace != "" && depref.Namespace != depns {
				continue
			}

			if depref.Namespace == "" && hdpl.Namespace != depns {
				continue
			}

			objkey := types.NamespacedName{
				Name:      hdpl.GetName(),
				Namespace: hdpl.GetNamespace(),
			}

			requests = append(requests, reconcile.Request{NamespacedName: objkey})
		}
	}

	return requests
}

type placementruleMapper struct {
	client.Client
}

func (mapper *placementruleMapper) Map(obj handler.MapObject) []reconcile.Request {
	var requests []reconcile.Request

	hdplList := &appv1alpha1.HybridDeployableList{}

	err := mapper.List(context.TODO(), hdplList, &client.ListOptions{})
	if err != nil {
		klog.Error("Failed to list hybrid deployables for deployerset with error:", err)
		return requests
	}

	prname := obj.Meta.GetName()
	prnamespace := obj.Meta.GetNamespace()

	for _, hdpl := range hdplList.Items {
		if hdpl.Spec.Placement != nil && hdpl.Spec.Placement.PlacementRef != nil && hdpl.Spec.Placement.PlacementRef.Name == prname {
			pref := hdpl.Spec.Placement.PlacementRef
			if pref.Namespace != "" && pref.Namespace != prnamespace {
				continue
			}

			if pref.Namespace == "" && prnamespace != hdpl.Namespace {
				continue
			}

			cp4mcmprgvk := schema.GroupVersionKind{
				Group:   "app.ibm.com",
				Version: "v1alpha1",
				Kind:    "PlacementRule",
			}
			if !pref.GroupVersionKind().Empty() && pref.GroupVersionKind().String() != cp4mcmprgvk.String() {
				continue
			}

			objkey := types.NamespacedName{
				Name:      hdpl.GetName(),
				Namespace: hdpl.GetNamespace(),
			}

			requests = append(requests, reconcile.Request{NamespacedName: objkey})
		}
	}

	return requests
}

type deployersetMapper struct {
	client.Client
}

func (mapper *deployersetMapper) Map(obj handler.MapObject) []reconcile.Request {
	var requests []reconcile.Request

	hdplList := &appv1alpha1.HybridDeployableList{}

	err := mapper.List(context.TODO(), hdplList, &client.ListOptions{})
	if err != nil {
		klog.Error("Failed to list hybrid deployables for deployerset with error:", err)
		return requests
	}

	for _, hdpl := range hdplList.Items {
		// only reconcile with when placement is set and not using ref
		if hdpl.Spec.Placement == nil {
			continue
		}

		if hdpl.Spec.Placement.PlacementRef != nil {
			objkey := types.NamespacedName{
				Name:      hdpl.GetName(),
				Namespace: hdpl.GetNamespace(),
			}

			requests = append(requests, reconcile.Request{NamespacedName: objkey})
		}
	}

	return requests
}

type deployerMapper struct {
	client.Client
}

func (mapper *deployerMapper) Map(obj handler.MapObject) []reconcile.Request {
	var requests []reconcile.Request

	hdplList := &appv1alpha1.HybridDeployableList{}
	err := mapper.List(context.TODO(), hdplList, &client.ListOptions{})

	if err != nil {
		klog.Error("Failed to list hybrid deployables for deployerset with error:", err)
		return requests
	}

	dplyname := obj.Meta.GetName()
	dplynamespace := obj.Meta.GetNamespace()

	for _, hdpl := range hdplList.Items {
		// only reconcile with when placement is set and not using ref
		if hdpl.Spec.Placement == nil {
			continue
		}

		if hdpl.Spec.Placement.PlacementRef == nil && hdpl.Spec.Placement.Deployers != nil {
			matched := false

			for _, cn := range hdpl.Spec.Placement.Deployers {
				if cn.Name == dplyname && cn.Namespace == dplynamespace {
					matched = true
				}
			}

			if !matched {
				continue
			}
		}

		objkey := types.NamespacedName{
			Name:      hdpl.GetName(),
			Namespace: hdpl.GetNamespace(),
		}

		requests = append(requests, reconcile.Request{NamespacedName: objkey})
	}

	return requests
}

type outputMapper struct {
	client.Client
}

func (mapper *outputMapper) Map(obj handler.MapObject) []reconcile.Request {
	var requests []reconcile.Request

	annotations := obj.Meta.GetAnnotations()
	if annotations == nil {
		return nil
	}

	var hdplkey string

	var ok bool

	if hdplkey, ok = annotations[appv1alpha1.OutputOf]; !ok {
		return nil
	}

	nn := strings.Split(hdplkey, "/")
	key := types.NamespacedName{
		Namespace: nn[0],
		Name:      nn[1],
	}

	requests = append(requests, reconcile.Request{NamespacedName: key})

	return requests
}

// ReconcileHybridDeployable reconciles a HybridDeployable object
type ReconcileHybridDeployable struct {
	// This client, initialized using mgr.Client() above, is a split client
	// that reads objects from the cache and writes to the apiserver
	client.Client
	eventRecorder *utils.EventRecorder
	hybridDeployableRegistry
}

// Reconcile reads that state of the cluster for a HybridDeployable object and makes changes based on the state read
// and what is in the HybridDeployable.Spec
// Note:
// The Controller will requeue the Request to be processed again if the returned error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (r *ReconcileHybridDeployable) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	klog.Info("Reconciling HybridDeployable:", request)

	// Fetch the HybridDeployable instance
	instance := &appv1alpha1.HybridDeployable{}

	err := r.Get(context.TODO(), request.NamespacedName, instance)
	if err != nil {
		if errors.IsNotFound(err) {
			children, err := r.getChildren(request.NamespacedName)
			if err == nil {
				r.purgeChildren(children)
			}

			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return reconcile.Result{}, err
	}

	deployers, err := r.getDeployersByPlacement(instance)
	if err != nil {
		klog.Error("Failed to get deployers from hybrid deployable spec placement with error:", err)
		return reconcile.Result{}, nil
	}

	children, err := r.getChildren(request.NamespacedName)
	if err != nil {
		klog.Error("Failed to get existing objects for hybriddeployable with error:", err)
	}

	instance.Status.PerDeployerStatus = nil

	r.deployResourceByDeployers(instance, deployers, children)

	r.purgeChildren(children)

	err = r.updateStatus(instance)
	if err != nil {
		klog.Info("Failed to update status with error:", err)
	}

	return reconcile.Result{}, nil
}
