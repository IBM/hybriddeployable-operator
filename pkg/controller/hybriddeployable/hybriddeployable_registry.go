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
	"time"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog"

	appv1alpha1 "github.com/IBM/hybriddeployable-operator/pkg/apis/app/v1alpha1"
)

var (
	resync            = 10 * time.Minute
	resourcePredicate = discovery.SupportsAllVerbs{Verbs: []string{"create", "update", "delete", "list", "watch"}}
)

type hybridDeployableRegistry struct {
	dynamicClient  dynamic.Interface
	dynamicFactory dynamicinformer.DynamicSharedInformerFactory
	gvkGVRMap      map[schema.GroupVersionKind]schema.GroupVersionResource
	activeGVRMap   map[schema.GroupVersionResource]schema.GroupVersionKind

	stopCh chan struct{}
}

func (r *hybridDeployableRegistry) initRegistry(config *rest.Config) error {
	var err error

	r.dynamicClient, err = dynamic.NewForConfig(config)

	r.activeGVRMap = make(map[schema.GroupVersionResource]schema.GroupVersionKind)

	r.discoverResources(config)

	r.discoverActiveGVRs()

	// add deployable by default
	r.activeGVRMap[deployableGVR] = deployableGVK

	klog.V(packageInfoLogLevel).Info("Initial active GVRs:", r.activeGVRMap)

	return err
}

func (r *hybridDeployableRegistry) discoverActiveGVRs() {
	for gvk, gvr := range r.gvkGVRMap {
		keylabel := map[string]string{
			appv1alpha1.ControlledBy: appv1alpha1.HybridDeployableController,
		}

		objlist, err := r.dynamicClient.Resource(gvr).List(metav1.ListOptions{LabelSelector: labels.Set(keylabel).String()})
		if err != nil {
			klog.Info("Skipping error in listing resource. error:", err)
			continue
		}

		if len(objlist.Items) > 0 {
			r.activeGVRMap[gvr] = gvk
		}
	}
}

func (r *hybridDeployableRegistry) discoverResources(config *rest.Config) {
	klog.Info("Discovering cluster resources")

	resources, err := discovery.NewDiscoveryClientForConfigOrDie(config).ServerPreferredResources()
	// do not return if there is error
	// some api server aggregation may cause this problem, but can still get return some resources.
	if err != nil {
		klog.Error("Failed to discover all server resources, continuing with err:", err)
	}

	filteredResources := discovery.FilteredBy(resourcePredicate, resources)

	klog.V(packageDetailLogLevel).Info("Discovered resources: ", filteredResources)

	r.gvkGVRMap = make(map[schema.GroupVersionKind]schema.GroupVersionResource)

	for _, rl := range filteredResources {
		r.buildGVKGVRMap(rl)
	}
}

func (r *hybridDeployableRegistry) buildGVKGVRMap(rl *metav1.APIResourceList) {
	for _, res := range rl.APIResources {
		gv, err := schema.ParseGroupVersion(rl.GroupVersion)
		if err != nil {
			klog.V(packageDetailLogLevel).Info("Skipping ", rl.GroupVersion, " with error:", err)
			continue
		}

		gvk := schema.GroupVersionKind{
			Kind:    res.Kind,
			Group:   gv.Group,
			Version: gv.Version,
		}
		gvr := schema.GroupVersionResource{
			Group:    gv.Group,
			Version:  gv.Version,
			Resource: res.Name,
		}

		r.gvkGVRMap[gvk] = gvr
	}
}

func (r *hybridDeployableRegistry) restart() {
	r.stop()

	r.stopCh = make(chan struct{})
	r.dynamicFactory = dynamicinformer.NewDynamicSharedInformerFactory(r.dynamicClient, resync)

	for gvr := range r.activeGVRMap {
		r.dynamicFactory.ForResource(gvr).Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
			AddFunc: func(new interface{}) {
			},
			UpdateFunc: func(old, new interface{}) {
			},
			DeleteFunc: func(old interface{}) {
			},
		})
	}

	r.dynamicFactory.Start(r.stopCh)
}

func (r *hybridDeployableRegistry) stop() {
	if r.stopCh != nil {
		r.dynamicFactory.WaitForCacheSync(r.stopCh)
		close(r.stopCh)
	}

	r.stopCh = nil
}

func (r *hybridDeployableRegistry) registerGVK(gvk schema.GroupVersionKind) (schema.GroupVersionResource, error) {
	gvr, ok := r.gvkGVRMap[gvk]
	if !ok {
		return schema.GroupVersionResource{}, errors.NewBadRequest(gvk.String() + "is not discovered")
	}

	_, ok = r.activeGVRMap[gvr]
	if !ok {
		r.restart()
		r.activeGVRMap[gvr] = gvk
	}

	klog.V(packageDetailLogLevel).Info("Updated active GVK/GVR map:", r.activeGVRMap)

	return gvr, nil
}
