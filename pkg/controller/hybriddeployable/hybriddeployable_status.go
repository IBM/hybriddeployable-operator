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

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog"
	"sigs.k8s.io/controller-runtime/pkg/client"

	dplv1alpha1 "github.com/IBM/multicloud-operators-deployable/pkg/apis/app/v1alpha1"

	deployerv1alpha1 "github.com/IBM/deployer-operator/pkg/apis/app/v1alpha1"
	deployerutils "github.com/IBM/deployer-operator/pkg/utils"
	appv1alpha1 "github.com/IBM/hybriddeployable-operator/pkg/apis/app/v1alpha1"
)

func (r *ReconcileHybridDeployable) updateStatus(instance *appv1alpha1.HybridDeployable) error {
	return r.Status().Update(context.TODO(), instance)
}

func (r *ReconcileHybridDeployable) updatePerDeployerStatus(instance *appv1alpha1.HybridDeployable, deployer *deployerv1alpha1.Deployer) {
	var err error

	labelmap := map[string]string{
		appv1alpha1.OutputOf: instance.Name,
	}
	listopt := &client.ListOptions{
		Namespace:     deployer.Namespace,
		LabelSelector: labels.Set(labelmap).AsSelector(),
	}

	dplystatus := appv1alpha1.PerDeployerStatus{}

	if deployerutils.IsInClusterDeployer(deployer) {
		epslist := &corev1.EndpointsList{}

		err = r.List(context.TODO(), epslist, listopt)
		if err != nil {
			klog.Info("Failed to list endpoints with error", err)
			return
		}

		for _, eps := range epslist.Items {
			ref := corev1.ObjectReference{}
			ref.SetGroupVersionKind(eps.GetObjectKind().GroupVersionKind())
			ref.Namespace = eps.Namespace
			ref.Name = eps.Name
			dplystatus.Outputs = append(dplystatus.Outputs, ref)
		}

		cfgmaplist := &corev1.ConfigMapList{}

		err = r.List(context.TODO(), cfgmaplist, listopt)
		if err != nil {
			klog.Info("Failed to list endpoints with error", err)
			return
		}

		for _, cfgmap := range cfgmaplist.Items {
			ref := corev1.ObjectReference{}
			ref.SetGroupVersionKind(cfgmap.GetObjectKind().GroupVersionKind())
			ref.Namespace = cfgmap.Namespace
			ref.Name = cfgmap.Name
			dplystatus.Outputs = append(dplystatus.Outputs, ref)
		}

		seclist := &corev1.SecretList{}

		err = r.List(context.TODO(), seclist, listopt)
		if err != nil {
			klog.Info("Failed to list endpoints with error", err)
			return
		}

		for _, sec := range seclist.Items {
			ref := corev1.ObjectReference{}
			ref.SetGroupVersionKind(sec.GetObjectKind().GroupVersionKind())
			ref.Namespace = sec.Namespace
			ref.Name = sec.Name
			dplystatus.Outputs = append(dplystatus.Outputs, ref)
		}
	} else {
		dpllist := &dplv1alpha1.DeployableList{}
		err = r.List(context.TODO(), dpllist, listopt)
		if err != nil {
			klog.Info("Failed to list endpoints with error", err)
			return
		}

		for _, dpl := range dpllist.Items {
			ref := corev1.ObjectReference{}
			ref.SetGroupVersionKind(deployableGVK)
			ref.Namespace = dpl.Namespace
			ref.Name = dpl.Name
			dplystatus.Outputs = append(dplystatus.Outputs, ref)
		}
	}

	if instance.Status.PerDeployerStatus == nil {
		instance.Status.PerDeployerStatus = make(map[string]appv1alpha1.PerDeployerStatus)
	}

	key := types.NamespacedName{
		Namespace: deployer.Namespace,
		Name:      deployer.Name,
	}

	instance.Status.PerDeployerStatus[key.String()] = dplystatus
}
