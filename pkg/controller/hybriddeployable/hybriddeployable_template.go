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
	"encoding/json"

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

var (
	deployableGVR = schema.GroupVersionResource{
		Group:    "app.ibm.com",
		Version:  "v1alpha1",
		Resource: "deployables",
	}
	deployableGVK = schema.GroupVersionKind{
		Group:   "app.ibm.com",
		Version: "v1alpha1",
		Kind:    "Deployable",
	}
	hybriddeployableGVK = schema.GroupVersionKind{
		Group:   appv1alpha1.SchemeGroupVersion.Group,
		Version: appv1alpha1.SchemeGroupVersion.Version,
		Kind:    "HybridDeployable",
	}
)

type gvrChildrenMap map[types.NamespacedName]metav1.Object

func (r *ReconcileHybridDeployable) purgeChildren(children map[schema.GroupVersionResource]gvrChildrenMap) {
	var err error

	for gvr, gvrchildren := range children {
		klog.V(packageDetailLogLevel).Info("Deleting obsolete children in gvr:", gvr)

		for k, obj := range gvrchildren {
			deletePolicy := metav1.DeletePropagationBackground

			err = r.dynamicClient.Resource(gvr).Namespace(obj.GetNamespace()).Delete(obj.GetName(), &metav1.DeleteOptions{PropagationPolicy: &deletePolicy})
			rtobj := obj.(runtime.Object)
			r.eventRecorder.RecordEvent(obj.(runtime.Object), "Delete", "Delete Object "+
				rtobj.GetObjectKind().GroupVersionKind().String()+" "+obj.GetNamespace()+"/"+obj.GetName(), err)

			if err != nil {
				klog.Error("Failed to delete obsolete child for key:", k, "with error:", err)
			}
		}
	}
}

func (r *ReconcileHybridDeployable) deployResourceByDeployers(instance *appv1alpha1.HybridDeployable, deployers []*deployerv1alpha1.Deployer,
	children map[schema.GroupVersionResource]gvrChildrenMap) {
	if instance == nil || instance.Spec.HybridTemplates == nil || deployers == nil {
		return
	}

	// prepare map to ease the search of template
	klog.V(packageDetailLogLevel).Info("Building template map:", instance.Spec.HybridTemplates)

	tplmap := make(map[string]*runtime.RawExtension)

	for _, tpl := range instance.Spec.HybridTemplates {
		tpltype := tpl.DeployerType
		if tpltype == "" {
			tpltype = appv1alpha1.DefaultDeployerType
		}

		tplmap[tpltype] = tpl.Template.DeepCopy()
	}

	for _, deployer := range deployers {
		template, ok := tplmap[deployer.Spec.Type]
		if !ok {
			continue
		}

		err := r.deployResourceByDeployer(instance, deployer, children, template)
		if err != nil {
			klog.Error("Failed to deploy resource by deployer, got error: ", err)
		}
	}
}

func (r *ReconcileHybridDeployable) deployResourceByDeployer(instance *appv1alpha1.HybridDeployable, deployer *deployerv1alpha1.Deployer,
	children map[schema.GroupVersionResource]gvrChildrenMap, template *runtime.RawExtension) error {
	obj := &unstructured.Unstructured{}

	err := json.Unmarshal(template.Raw, obj)
	if err != nil {
		klog.Info("Failed to unmashal object:\n", string(template.Raw))
		return err
	}

	gvk := obj.GetObjectKind().GroupVersionKind()

	gvr, err := r.registerGVK(gvk)
	if err != nil {
		return err
	}

	if !deployerutils.IsInClusterDeployer(deployer) {
		gvr = deployableGVR
	}

	gvrchildren := children[gvr]
	if gvrchildren == nil {
		gvrchildren = make(gvrChildrenMap)
	}

	var metaobj metav1.Object

	var key types.NamespacedName

	for k, child := range gvrchildren {
		if child.GetNamespace() != deployer.GetNamespace() {
			continue
		}

		anno := child.GetAnnotations()
		if anno != nil && anno[appv1alpha1.DependencyFrom] == "" {
			metaobj = child
			key = k

			break
		}
	}

	klog.V(packageDetailLogLevel).Info("Checking gvr:", gvr, " child key:", key, "object:", metaobj)

	templateobj := &unstructured.Unstructured{}

	if template.Object != nil {
		uc, err := runtime.DefaultUnstructuredConverter.ToUnstructured(template.Object)
		if err != nil {
			klog.Error("Failed to convert template object to unstructured with error: ", err)
			return err
		}

		templateobj.SetUnstructuredContent(uc)
	} else {
		err = json.Unmarshal(template.Raw, templateobj)
		if err != nil {
			klog.Info("Failed to unmashal object with error", err, ". Raw :\n", string(template.Raw))
			return nil
		}
	}

	_, err = r.deployObjectForDeployer(instance, deployer, metaobj, templateobj)
	if err == nil && metaobj != nil {
		delete(gvrchildren, key)
	}

	children[gvr] = gvrchildren
	klog.V(packageDetailLogLevel).Info("gvr children:", gvrchildren)

	r.deployDependenciesByDeployer(instance, deployer, children, templateobj)
	r.updatePerDeployerStatus(instance, deployer)

	return nil
}

func (r *ReconcileHybridDeployable) deployObjectForDeployer(instance *appv1alpha1.HybridDeployable, deployer *deployerv1alpha1.Deployer,
	object metav1.Object, templateobj *unstructured.Unstructured) (metav1.Object, error) {
	var err error

	// generate deployable
	klog.V(packageDetailLogLevel).Info("Processing Deployable for deployer type ", deployer.Spec.Type, ": ", templateobj)

	if object != nil {
		klog.V(packageDetailLogLevel).Info("Updating Object for Object:", object.GetNamespace(), "/", object.GetName())

		object, err = r.updateObjectForDeployer(templateobj, object)
		if err != nil {
			klog.Error("Failed to update object with error: ", err)
			return nil, err
		}

		rtobj := object.(runtime.Object)
		r.eventRecorder.RecordEvent(object.(runtime.Object), "Update", "Update Object "+
			rtobj.GetObjectKind().GroupVersionKind().String()+" "+object.GetNamespace()+"/"+object.GetName(), err)
	} else {
		object, err = r.createObjectForDeployer(instance, deployer, templateobj)
		if err != nil {
			klog.Error("Failed to create new object with error: ", err)
			return nil, err
		}
		rtobj := object.(runtime.Object)
		r.eventRecorder.RecordEvent(object.(runtime.Object), "Create", "Create Object "+
			rtobj.GetObjectKind().GroupVersionKind().String()+" "+object.GetNamespace()+"/"+object.GetName(), err)
	}

	if err != nil {
		klog.Error("Failed to process object with error:", err)
	}

	return object, err
}

func (r *ReconcileHybridDeployable) updateObjectForDeployer(templateobj *unstructured.Unstructured, object metav1.Object) (metav1.Object, error) {
	uc, err := runtime.DefaultUnstructuredConverter.ToUnstructured(object)
	if err != nil {
		klog.Error("Failed to convert deployable to unstructured with error:", err)
		return nil, err
	}

	obj := &unstructured.Unstructured{}
	obj.SetUnstructuredContent(uc)
	gvk := obj.GetObjectKind().GroupVersionKind()
	gvr, err := r.registerGVK(gvk)

	if err != nil {
		klog.Error("Failed to obtain right gvr for gvk ", gvk, " with error: ", err)
		return nil, err
	}

	// handle the deployable
	if gvr == deployableGVR {
		dpl := &dplv1alpha1.Deployable{}
		err = runtime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, dpl)

		if err != nil {
			klog.Error("Failed to convert deployable to unstructured with error:", err)
			return nil, err
		}

		dpl.Spec.Template = &runtime.RawExtension{
			Object: templateobj,
		}
		uc, err := runtime.DefaultUnstructuredConverter.ToUnstructured(dpl)

		if err != nil {
			klog.Error("Failed to convert deployable to unstructured with error:", err)
			return nil, err
		}

		obj.SetUnstructuredContent(uc)
	} else {
		newobj := templateobj.DeepCopy()
		newobj.SetNamespace(obj.GetNamespace())
		newobj.SetAnnotations(obj.GetAnnotations())
		newobj.SetLabels(obj.GetLabels())
		obj = newobj
	}

	klog.V(packageDetailLogLevel).Info("Updating Object:", obj)

	return r.dynamicClient.Resource(gvr).Namespace(obj.GetNamespace()).Update(obj, metav1.UpdateOptions{})
}

func (r *ReconcileHybridDeployable) createObjectForDeployer(instance *appv1alpha1.HybridDeployable, deployer *deployerv1alpha1.Deployer,
	templateobj *unstructured.Unstructured) (metav1.Object, error) {
	gvk := templateobj.GetObjectKind().GroupVersionKind()

	gvr, err := r.registerGVK(gvk)
	if err != nil {
		klog.Error("Failed to obtain right gvr for gvk ", gvk, " with error: ", err)
		return nil, err
	}

	obj := templateobj.DeepCopy()
	// actual object to be created could be template object or a deployable wrapping template object
	if !deployerutils.IsInClusterDeployer(deployer) {
		dpl := &dplv1alpha1.Deployable{}

		r.prepareUnstructured(instance, templateobj)

		dpl.Spec.Template = &runtime.RawExtension{
			Object: templateobj,
		}
		uc, err := runtime.DefaultUnstructuredConverter.ToUnstructured(dpl)

		if err != nil {
			klog.Error("Failed to convert deployable to unstructured with error:", err)
			return nil, err
		}

		gvr = deployableGVR

		obj.SetUnstructuredContent(uc)
		obj.SetGroupVersionKind(deployableGVK)
	}

	if obj.GetName() == "" {
		obj.SetGenerateName(r.genDeployableGenerateName(instance, deployer))
	}

	obj.SetNamespace(deployer.Namespace)
	r.prepareUnstructured(instance, obj)

	klog.V(packageDetailLogLevel).Info("Creating Object:", obj)

	return r.dynamicClient.Resource(gvr).Namespace(deployer.Namespace).Create(obj, metav1.CreateOptions{})
}

func (r *ReconcileHybridDeployable) genDeployableGenerateName(instance *appv1alpha1.HybridDeployable, deployer *deployerv1alpha1.Deployer) string {
	return instance.Name + "-" + deployer.Spec.Type + "-"
}

func (r *ReconcileHybridDeployable) genObjectIdentifier(metaobj metav1.Object) types.NamespacedName {
	id := types.NamespacedName{
		Namespace: metaobj.GetNamespace(),
	}

	annotations := metaobj.GetAnnotations()
	if annotations != nil && (annotations[appv1alpha1.HostingHybridDeployable] != "" || annotations[deployerv1alpha1.HostingDeployer] != "") {
		id.Name = annotations[appv1alpha1.HostingHybridDeployable] + "-" + annotations[deployerv1alpha1.HostingDeployer] + "-"
	}

	if metaobj.GetGenerateName() != "" {
		id.Name += metaobj.GetGenerateName()
	} else {
		id.Name += metaobj.GetName()
	}

	return id
}

func (r *ReconcileHybridDeployable) prepareUnstructured(instance *appv1alpha1.HybridDeployable, object *unstructured.Unstructured) {
	labels := object.GetLabels()
	if labels == nil {
		labels = make(map[string]string)
	}

	labels[appv1alpha1.HostingHybridDeployable] = instance.Name
	labels[appv1alpha1.ControlledBy] = appv1alpha1.HybridDeployableController

	object.SetLabels(labels)

	annotations := object.GetAnnotations()
	if annotations == nil {
		annotations = make(map[string]string)
	}

	annotations[appv1alpha1.HostingHybridDeployable] = types.NamespacedName{Namespace: instance.Namespace, Name: instance.Name}.String()

	object.SetAnnotations(annotations)
}
