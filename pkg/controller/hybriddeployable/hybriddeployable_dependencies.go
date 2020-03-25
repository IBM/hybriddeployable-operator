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
	"encoding/json"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog"

	dplv1alpha1 "github.com/IBM/multicloud-operators-deployable/pkg/apis/app/v1alpha1"

	deployerv1alpha1 "github.com/IBM/deployer-operator/pkg/apis/app/v1alpha1"
	deployerutils "github.com/IBM/deployer-operator/pkg/utils"
	appv1alpha1 "github.com/IBM/hybriddeployable-operator/pkg/apis/app/v1alpha1"
)

func (r *ReconcileHybridDeployable) getDependenciesObjectReferences(instance *appv1alpha1.HybridDeployable) []corev1.ObjectReference {
	var objrefs []corev1.ObjectReference

	var err error

	for _, ref := range instance.Spec.Dependencies {
		if ref.GetObjectKind().GroupVersionKind() == hybriddeployableGVK {
			hdpl := &appv1alpha1.HybridDeployable{}
			key := types.NamespacedName{Name: ref.Name, Namespace: ref.Namespace}

			if key.Namespace == "" {
				key.Namespace = instance.Namespace
			}

			err = r.Get(context.TODO(), key, hdpl)
			if err != nil {
				klog.Error("Failed to obtain hybriddeployable dependency with error:", err)
				continue
			}

			for _, deployerstatus := range hdpl.Status.PerDeployerStatus {
				// Add all of them for now
				objrefs = append(objrefs, deployerstatus.Outputs...)
			}
		} else {
			refitem := corev1.ObjectReference{}
			ref.DeepCopyInto(&refitem)
			objrefs = append(objrefs, refitem)
		}
	}

	return objrefs
}

func (r *ReconcileHybridDeployable) getDependenciesObject(instance *appv1alpha1.HybridDeployable, depref corev1.ObjectReference) *unstructured.Unstructured {
	var depobj *unstructured.Unstructured

	var err error

	depgvr, ok := r.gvkGVRMap[depref.GetObjectKind().GroupVersionKind()]
	if !ok {
		klog.Info("Failed to obtain gvr for dependency gvk:", depref.GetObjectKind().GroupVersionKind())
		return nil
	}

	if depref.GetObjectKind().GroupVersionKind() == deployableGVK {
		dplobj := &dplv1alpha1.Deployable{}
		key := types.NamespacedName{Name: depref.Name, Namespace: depref.Namespace}

		err = r.Get(context.TODO(), key, dplobj)
		if err != nil {
			klog.Info("Failed to obtain deployable dependency with error:", err)
			return nil
		}

		if dplobj.Spec.Template == nil {
			return nil
		}

		depobj = &unstructured.Unstructured{}

		if dplobj.Spec.Template.Object != nil {
			uc, err := runtime.DefaultUnstructuredConverter.ToUnstructured(dplobj.Spec.Template.Object)
			if err != nil {
				klog.Info("Failed to convert deployable template object with error:", err)
				return nil
			}

			depobj.SetUnstructuredContent(uc)
		} else {
			err = json.Unmarshal(dplobj.Spec.Template.Raw, depobj)
			if err != nil {
				klog.Info("Failed to unmashal object:\n", string(dplobj.Spec.Template.Raw))
				return nil
			}
		}
	} else {
		depns := depref.Namespace
		if depns == "" {
			depns = instance.Namespace
		}
		depobj, err = r.dynamicClient.Resource(depgvr).Namespace(depns).Get(depref.Name, metav1.GetOptions{})
		if err != nil {
			klog.Info("Failed to obtain dependency object with error: ", err)
			return nil
		}
	}

	klog.V(packageDetailLogLevel).Info("Retrieved dependency object:", depobj)

	return depobj
}

func (r *ReconcileHybridDeployable) getDependenciesObjects(instance *appv1alpha1.HybridDeployable) []*unstructured.Unstructured {
	if instance == nil || instance.Spec.Dependencies == nil {
		return nil
	}

	var depobjects []*unstructured.Unstructured

	var depobj *unstructured.Unstructured

	objrefs := r.getDependenciesObjectReferences(instance)

	for _, depref := range objrefs {
		depobj = r.getDependenciesObject(instance, depref)
		if depobj == nil {
			continue
		}

		// cleanup obj for creation
		var emptyuid types.UID

		depobj.SetGroupVersionKind(depobj.GetObjectKind().GroupVersionKind())
		depobj.SetUID(emptyuid)
		depobj.SetSelfLink("")
		depobj.SetResourceVersion("")
		depobj.SetGeneration(0)
		depobj.SetCreationTimestamp(metav1.Time{})
		depobjects = append(depobjects, depobj.DeepCopy())
	}

	return depobjects
}

func (r *ReconcileHybridDeployable) deployDependenciesByDeployer(instance *appv1alpha1.HybridDeployable, deployer *deployerv1alpha1.Deployer,
	children map[schema.GroupVersionResource]gvrChildrenMap, templateobj *unstructured.Unstructured) {
	if deployer == nil {
		return
	}

	depobjects := r.getDependenciesObjects(instance)

	if depobjects == nil {
		return
	}

	klog.V(packageDetailLogLevel).Info("deploying dependencies for hybriddeployable ", instance.GetNamespace(),
		"/", instance.GetName(), ": ", instance.Spec.Dependencies)

	for _, depobj := range depobjects {
		depgvr, err := r.registerGVK(depobj.GetObjectKind().GroupVersionKind())
		if err != nil {
			klog.Info("Failed to obtain gvr for dependency gvk:", depobj.GetObjectKind().GroupVersionKind())
			continue
		}

		targetGVR := depgvr
		targetobj := depobj.DeepCopy()

		if !deployerutils.IsInClusterDeployer(deployer) {
			// make sure it go to same namespace with template in hybrid deployable spec
			depobj.SetNamespace(templateobj.GetNamespace())

			targetGVR = deployableGVR
			dplobj := &dplv1alpha1.Deployable{}
			dplobj.SetGroupVersionKind(deployableGVK)
			dplobj.Spec.Template = &runtime.RawExtension{}
			dplobj.Spec.Template.Object = depobj

			uc, err := runtime.DefaultUnstructuredConverter.ToUnstructured(dplobj)
			if err != nil {
				klog.Info("Failed to convert deployable to unstructured with error:", err)
				continue
			}

			targetobj.SetUnstructuredContent(uc)
			targetobj.SetName(depobj.GetName())
		}

		targetobj.SetNamespace(deployer.Namespace)
		r.prepareUnstructured(instance, targetobj)
		r.prepareUnstructuredAsDependency(depobj, targetobj)

		klog.V(packageDetailLogLevel).Info("Ready to deploy dependency:", targetobj)

		existing, err := r.dynamicClient.Resource(targetGVR).Namespace(deployer.Namespace).Get(depobj.GetName(), metav1.GetOptions{})
		if err != nil {
			if errors.IsNotFound(err) {
				_, err = r.dynamicClient.Resource(targetGVR).Namespace(deployer.Namespace).Create(targetobj, metav1.CreateOptions{})
				if err != nil {
					klog.Info("Failed to create new dependency object for deployer with error: ", err)
					continue
				}
			} else {
				klog.Info("Failed to obtain dependency object in deployer namespace with error: ", err)
				continue
			}
		} else {
			targetobj.SetCreationTimestamp(existing.GetCreationTimestamp())
			targetobj.SetUID(existing.GetUID())
			targetobj.SetGeneration(existing.GetGeneration())
			targetobj.SetResourceVersion(existing.GetResourceVersion())
			_, err = r.dynamicClient.Resource(targetGVR).Namespace(deployer.Namespace).Update(targetobj, metav1.UpdateOptions{})
			if err != nil {
				klog.Info("Failed to update existing dependency object for deployer with error: ", err)
			}
		}

		gvrchildren := children[targetGVR]
		if gvrchildren != nil {
			delete(gvrchildren, r.genObjectIdentifier(targetobj))
		}
	}
}

func (r *ReconcileHybridDeployable) prepareUnstructuredAsDependency(depobj, object *unstructured.Unstructured) {
	annotations := object.GetAnnotations()
	if annotations == nil {
		annotations = make(map[string]string)
	}

	annotations[appv1alpha1.DependencyFrom] = depobj.GetNamespace() + "/" + depobj.GetName()

	object.SetAnnotations(annotations)
}
