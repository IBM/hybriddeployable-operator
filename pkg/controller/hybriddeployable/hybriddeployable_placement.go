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

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	clusterv1alpha1 "k8s.io/cluster-registry/pkg/apis/clusterregistry/v1alpha1"
	"k8s.io/klog"

	"sigs.k8s.io/controller-runtime/pkg/client"

	placementv1alpha1 "github.com/IBM/multicloud-operators-placementrule/pkg/apis/app/v1alpha1"
	placementutils "github.com/IBM/multicloud-operators-placementrule/pkg/utils"

	deployerv1alpha1 "github.com/IBM/deployer-operator/pkg/apis/app/v1alpha1"
	deployerutils "github.com/IBM/deployer-operator/pkg/utils"
	appv1alpha1 "github.com/IBM/hybriddeployable-operator/pkg/apis/app/v1alpha1"
)

// Top priority: placementRef, ignore others
// Next priority: clusterNames, ignore selector
// Bottomline: Use label selector
func (r *ReconcileHybridDeployable) getDeployersByPlacement(instance *appv1alpha1.HybridDeployable) ([]*deployerv1alpha1.Deployer, error) {
	if instance == nil || instance.Spec.Placement == nil {
		return nil, nil
	}

	var deployers []*deployerv1alpha1.Deployer

	var err error

	if instance.Spec.Placement.PlacementRef != nil {
		deployers, err = r.getDeployersByPlacementReference(instance)
		return deployers, err
	}

	if instance.Spec.Placement.Deployers != nil {
		for _, dplyref := range instance.Spec.Placement.Deployers {
			deployer := &deployerv1alpha1.Deployer{}

			err = r.Get(context.TODO(), types.NamespacedName{Name: dplyref.Name, Namespace: dplyref.Namespace}, deployer)
			if err != nil {
				klog.V(packageDetailLogLevel).Info("Trying to obtain object from deployers list, but got error: ", err)
				continue
			}

			annotations := deployer.GetAnnotations()
			if annotations == nil {
				annotations = make(map[string]string)
			}

			deployer.SetAnnotations(annotations)

			deployerutils.SetInClusterDeployer(deployer)
			deployers = append(deployers, deployer)
		}

		return deployers, err
	}

	if instance.Spec.Placement.DeployerLabels != nil {
		deployers, err = r.getDeployersByLabelSelector(instance.Spec.Placement.DeployerLabels)
		return deployers, err
	}

	return nil, nil
}

func (r *ReconcileHybridDeployable) getDeployersByLabelSelector(deployerLabels *metav1.LabelSelector) ([]*deployerv1alpha1.Deployer, error) {
	clSelector, err := placementutils.ConvertLabels(deployerLabels)
	if err != nil {
		return nil, err
	}

	klog.V(packageDetailLogLevel).Info("Using Cluster LabelSelector ", clSelector)

	dplylist := &deployerv1alpha1.DeployerList{}

	err = r.List(context.TODO(), dplylist, &client.ListOptions{LabelSelector: clSelector})

	if err != nil && !errors.IsNotFound(err) {
		klog.Error("Listing clusters and found error: ", err)
		return nil, err
	}

	klog.V(packageDetailLogLevel).Info("listed deployers:", dplylist.Items)

	var deployers []*deployerv1alpha1.Deployer

	for _, dply := range dplylist.Items {
		deployer := dply.DeepCopy()

		annotations := deployer.GetAnnotations()
		if annotations == nil {
			annotations = make(map[string]string)
		}

		deployer.SetAnnotations(annotations)

		deployerutils.SetInClusterDeployer(deployer)
		deployers = append(deployers, deployer)
	}

	return deployers, err
}

func (r *ReconcileHybridDeployable) getDeployersByPlacementReference(instance *appv1alpha1.HybridDeployable) ([]*deployerv1alpha1.Deployer, error) {
	var deployers []*deployerv1alpha1.Deployer

	clustermap, err := r.getClusterMapByPlacementRule(instance)

	if err != nil {
		klog.Error("Failed find target namespaces with error: ", err)
		return nil, err
	}

	klog.V(packageDetailLogLevel).Info("Find clusters for hybrid deployable:", clustermap)

	for _, cl := range clustermap {
		dset := &deployerv1alpha1.DeployerSet{}
		key := types.NamespacedName{
			Namespace: cl.Namespace,
			Name:      cl.Name,
		}
		deployer := &deployerv1alpha1.Deployer{}
		deployer.Name = cl.Name
		deployer.Namespace = cl.Namespace
		deployer.Spec.Type = appv1alpha1.DefaultDeployerType

		err = r.Get(context.TODO(), key, dset)
		klog.V(packageDetailLogLevel).Info("Got Deployerset for cluster ", cl.GetNamespace(), "/", cl.GetName(), " with err:", err, " result: ", dset)

		if err != nil {
			if !errors.IsNotFound(err) {
				continue
			}
		} else {
			if len(dset.Spec.Deployers) > 0 {
				dset.Spec.Deployers[0].Spec.DeepCopyInto(&deployer.Spec)

				if dset.Spec.DefaultDeployer != "" {
					for _, dply := range dset.Spec.Deployers {
						if dply.Key == dset.Spec.DefaultDeployer {
							dply.Spec.DeepCopyInto(&deployer.Spec)
							break
						}
					}
				}
				klog.V(packageDetailLogLevel).Info("Copyied deployer info:", deployer)
			}
		}

		klog.V(packageDetailLogLevel).Info("Adding deployer: ", deployer)

		annotations := deployer.GetAnnotations()
		if annotations == nil {
			annotations = make(map[string]string)
		}

		deployer.SetAnnotations(annotations)
		deployerutils.SetRemoteDeployer(deployer)

		deployers = append(deployers, deployer)
	}

	klog.V(packageDetailLogLevel).Info("Deploying to deployers", deployers)

	return deployers, nil
}

func (r *ReconcileHybridDeployable) getChildren(request types.NamespacedName) (map[schema.GroupVersionResource]gvrChildrenMap, error) {
	children := make(map[schema.GroupVersionResource]gvrChildrenMap)

	nameLabel := map[string]string{
		appv1alpha1.HostingHybridDeployable: request.Name,
	}

	for gvr := range r.activeGVRMap {
		objlist, err := r.dynamicClient.Resource(gvr).List(metav1.ListOptions{LabelSelector: labels.Set(nameLabel).String()})
		klog.V(packageDetailLogLevel).Info("Processing active gvr", gvr, "and got list:", objlist.Items)

		if err != nil {
			return nil, err
		}

		gvkchildren := make(map[types.NamespacedName]metav1.Object)

		for _, obj := range objlist.Items {
			annotations := obj.GetAnnotations()
			if annotations == nil {
				continue
			}

			if host, ok := annotations[appv1alpha1.HostingHybridDeployable]; ok {
				if host == request.String() {
					key := r.genObjectIdentifier(&obj)

					if _, ok := gvkchildren[key]; ok {
						klog.Info("Deleting redundant deployed object", obj.GetNamespace(), "/", obj.GetName())

						deletePolicy := metav1.DeletePropagationBackground

						_ = r.dynamicClient.Resource(gvr).Namespace(obj.GetNamespace()).Delete(obj.GetName(), &metav1.DeleteOptions{PropagationPolicy: &deletePolicy})
					} else {
						gvkchildren[key] = obj.DeepCopy()
					}

					klog.V(packageDetailLogLevel).Info("Adding gvk child with key:", key)
				}
			}
		}

		children[gvr] = gvkchildren
	}

	return children, nil
}

func (r *ReconcileHybridDeployable) getClusterMapByPlacementRule(instance *appv1alpha1.HybridDeployable) (map[string]*clusterv1alpha1.Cluster, error) {
	if instance == nil || instance.Spec.Placement == nil || instance.Spec.Placement.PlacementRef == nil {
		return nil, nil
	}

	pref := instance.Spec.Placement.PlacementRef

	// only default to mcm placementrule, and only support mcm placementrule now
	if len(pref.Kind) > 0 && pref.Kind != "PlacementRule" || len(pref.APIVersion) > 0 && pref.APIVersion != "app.ibm.com/v1alpha1" {
		klog.Warning("Unsupported placement reference:", pref)

		return nil, nil
	}

	clustermap := make(map[string]*clusterv1alpha1.Cluster)

	klog.V(packageDetailLogLevel).Info("Referencing existing PlacementRule:", pref)

	// get placementpolicy resource
	pp := &placementv1alpha1.PlacementRule{}
	pkey := types.NamespacedName{
		Name:      pref.Name,
		Namespace: pref.Namespace,
	}

	if pref.Namespace == "" {
		pkey.Namespace = instance.Namespace
	}

	err := r.Get(context.TODO(), pkey, pp)
	if err != nil {
		if errors.IsNotFound(err) {
			klog.Warning("Failed to locate placement reference", instance.Spec.Placement.PlacementRef)

			return nil, nil
		}

		return nil, err
	}

	klog.V(packageDetailLogLevel).Info("Preparing cluster namespaces from ", pp)

	for _, decision := range pp.Status.Decisions {
		cluster := &clusterv1alpha1.Cluster{}
		cluster.Name = decision.ClusterName
		cluster.Namespace = decision.ClusterNamespace
		clustermap[decision.ClusterName] = cluster
	}

	return clustermap, nil
}
