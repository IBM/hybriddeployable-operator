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
	"testing"
	"time"

	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/types"
	clusterv1alpha1 "k8s.io/cluster-registry/pkg/apis/clusterregistry/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	deployerv1alpha1 "github.com/IBM/deployer-operator/pkg/apis/app/v1alpha1"
	hybriddeployablev1alpha1 "github.com/IBM/hybriddeployable-operator/pkg/apis/app/v1alpha1"

	deployablev1alpha1 "github.com/IBM/multicloud-operators-deployable/pkg/apis/app/v1alpha1"
	placementv1alpha1 "github.com/IBM/multicloud-operators-placementrule/pkg/apis/app/v1alpha1"
)

const timeout = time.Second * 30
const interval = time.Second * 1

var (
	clusterName      = "cluster-1"
	clusterNamespace = "cluster-1"

	cluster = &clusterv1alpha1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      clusterName,
			Namespace: clusterNamespace,
		},
	}

	deployerName      = "mydplyr"
	deployerNamespace = clusterNamespace
	deployerKey       = types.NamespacedName{
		Name:      deployerName,
		Namespace: deployerNamespace,
	}

	deployerType = "configmap"

	deployer = &deployerv1alpha1.Deployer{
		ObjectMeta: metav1.ObjectMeta{
			Name:      deployerKey.Name,
			Namespace: deployerKey.Namespace,
			Labels:    map[string]string{"deployer-type": deployerType},
		},
		Spec: deployerv1alpha1.DeployerSpec{
			Type: deployerType,
		},
	}

	deployerSetKey = types.NamespacedName{
		Name:      clusterName,
		Namespace: clusterNamespace,
	}

	deployerInSetSpec = deployerv1alpha1.DeployerSpecDescriptor{
		Key: "default/mydplyr",
		Spec: deployerv1alpha1.DeployerSpec{
			Type: deployerType,
		},
	}

	deployerSet = &deployerv1alpha1.DeployerSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      clusterName,
			Namespace: deployerSetKey.Namespace,
		},
		Spec: deployerv1alpha1.DeployerSetSpec{
			Deployers: []deployerv1alpha1.DeployerSpecDescriptor{
				deployerInSetSpec,
			},
		},
	}

	placementRuleName      = "prule-1"
	placementRuleNamespace = "default"
	placementRuleKey       = types.NamespacedName{
		Name:      placementRuleName,
		Namespace: placementRuleNamespace,
	}
	placementRule = &placementv1alpha1.PlacementRule{
		ObjectMeta: metav1.ObjectMeta{
			Name:      placementRuleName,
			Namespace: placementRuleNamespace,
		},
		Spec: placementv1alpha1.PlacementRuleSpec{
			GenericPlacementFields: placementv1alpha1.GenericPlacementFields{
				ClusterSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{"name": clusterName},
				},
			},
		},
	}

	payloadFoo = &corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "ConfigMap",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "payload",
		},
		Data: map[string]string{"myconfig": "foo"},
	}

	payloadBar = &corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "ConfigMap",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "payload",
		},
		Data: map[string]string{"myconfig": "bar"},
	}

	hybridDeployableName      = "test-hd"
	hybridDeployableNamespace = "test-hd-ns"
	hybridDeployableKey       = types.NamespacedName{
		Name:      hybridDeployableName,
		Namespace: hybridDeployableNamespace,
	}

	hybridDeployable = &hybriddeployablev1alpha1.HybridDeployable{
		ObjectMeta: metav1.ObjectMeta{
			Name:      hybridDeployableName,
			Namespace: hybridDeployableNamespace,
		},
		Spec: hybriddeployablev1alpha1.HybridDeployableSpec{},
	}

	expectedRequest = reconcile.Request{NamespacedName: hybridDeployableKey}
)

func TestReconcileWithDeployer(t *testing.T) {
	g := NewWithT(t)

	templateInHybridDeployable := hybriddeployablev1alpha1.HybridTemplate{
		DeployerType: deployerType,
		Template: &runtime.RawExtension{
			Object: payloadFoo,
		},
	}

	deployerInPlacement := corev1.ObjectReference{
		Name:      deployerKey.Name,
		Namespace: deployerKey.Namespace,
	}

	hybridDeployable.Spec = hybriddeployablev1alpha1.HybridDeployableSpec{
		HybridTemplates: []hybriddeployablev1alpha1.HybridTemplate{
			templateInHybridDeployable,
		},
		Placement: &hybriddeployablev1alpha1.HybridPlacement{
			Deployers: []corev1.ObjectReference{
				deployerInPlacement,
			},
		},
	}

	var c client.Client

	// Setup the Manager and Controller.  Wrap the Controller Reconcile function so it writes each request to a
	// channel when it is finished.
	mgr, err := manager.New(cfg, manager.Options{MetricsBindAddress: "0"})
	g.Expect(err).NotTo(HaveOccurred())

	c = mgr.GetClient()

	rec := newReconciler(mgr)
	recFn, requests := SetupTestReconcile(rec)

	g.Expect(add(mgr, recFn)).To(Succeed())

	stopMgr, mgrStopped := StartTestManager(mgr, g)

	defer func() {
		close(stopMgr)
		mgrStopped.Wait()
	}()

	dplyr := deployer.DeepCopy()
	g.Expect(c.Create(context.TODO(), dplyr)).To(Succeed())
	defer c.Delete(context.TODO(), dplyr)

	//Expect  payload is created in deplyer namespace on hybriddeployable create
	instance := hybridDeployable.DeepCopy()
	g.Expect(c.Create(context.TODO(), instance)).To(Succeed())
	g.Eventually(requests, timeout, interval).Should(Receive(Equal(expectedRequest)))

	pl := &corev1.ConfigMap{}
	plKey := types.NamespacedName{
		Name:      payloadFoo.Name,
		Namespace: deployer.Namespace,
	}
	g.Expect(c.Get(context.TODO(), plKey, pl)).To(Succeed())

	//status update reconciliation
	g.Eventually(requests, timeout, interval).Should(Receive(Equal(expectedRequest)))

	//Expect payload is updated on hybriddeployable template update
	instance = &hybriddeployablev1alpha1.HybridDeployable{}
	g.Expect(c.Get(context.TODO(), hybridDeployableKey, instance)).To(Succeed())
	instance.Spec.HybridTemplates[0].Template = &runtime.RawExtension{Object: payloadBar}

	g.Expect(c.Update(context.TODO(), instance)).To(Succeed())
	g.Eventually(requests, timeout, interval).Should(Receive(Equal(expectedRequest)))

	plKey = types.NamespacedName{
		Name:      payloadBar.Name,
		Namespace: deployer.Namespace,
	}
	pl = &corev1.ConfigMap{}
	g.Expect(c.Get(context.TODO(), plKey, pl)).To(Succeed())
	defer c.Delete(context.TODO(), pl)
	g.Expect(pl.Data).To(Equal(payloadBar.Data))

	//Expect payload ro be removed on hybriddeployable delete
	c.Delete(context.TODO(), instance)
	g.Eventually(requests, timeout, interval).Should(Receive(Equal(expectedRequest)))
	g.Expect(c.Get(context.TODO(), plKey, pl)).NotTo(Succeed())
}

func TestReconcileWithDeployerLabel(t *testing.T) {
	g := NewWithT(t)

	templateInHybridDeployable := hybriddeployablev1alpha1.HybridTemplate{
		DeployerType: deployerType,
		Template: &runtime.RawExtension{
			Object: payloadFoo,
		},
	}

	hybridDeployable.Spec = hybriddeployablev1alpha1.HybridDeployableSpec{
		HybridTemplates: []hybriddeployablev1alpha1.HybridTemplate{
			templateInHybridDeployable,
		},
		Placement: &hybriddeployablev1alpha1.HybridPlacement{
			DeployerLabels: &metav1.LabelSelector{
				MatchLabels: map[string]string{"deployer-type": deployerType},
			},
		},
	}

	var c client.Client

	// Setup the Manager and Controller.  Wrap the Controller Reconcile function so it writes each request to a
	// channel when it is finished.
	mgr, err := manager.New(cfg, manager.Options{MetricsBindAddress: "0"})
	g.Expect(err).NotTo(HaveOccurred())

	c = mgr.GetClient()

	rec := newReconciler(mgr)
	recFn, requests := SetupTestReconcile(rec)

	g.Expect(add(mgr, recFn)).To(Succeed())

	stopMgr, mgrStopped := StartTestManager(mgr, g)

	defer func() {
		close(stopMgr)
		mgrStopped.Wait()
	}()

	dplyr := deployer.DeepCopy()
	g.Expect(c.Create(context.TODO(), dplyr)).To(Succeed())
	defer c.Delete(context.TODO(), dplyr)

	//Expect  payload is created in deplyer namespace on hybriddeployable create

	instance := hybridDeployable.DeepCopy()
	g.Expect(c.Create(context.TODO(), instance)).To(Succeed())
	g.Eventually(requests, timeout, interval).Should(Receive(Equal(expectedRequest)))

	pl := &corev1.ConfigMap{}
	plKey := types.NamespacedName{
		Name:      payloadFoo.Name,
		Namespace: deployer.Namespace,
	}
	g.Expect(c.Get(context.TODO(), plKey, pl)).To(Succeed())

	//status update reconciliation
	g.Eventually(requests, timeout, interval).Should(Receive(Equal(expectedRequest)))

	//Expect payload is updated on hybriddeployable template update
	instance = &hybriddeployablev1alpha1.HybridDeployable{}
	g.Expect(c.Get(context.TODO(), hybridDeployableKey, instance)).To(Succeed())
	instance.Spec.HybridTemplates[0].Template = &runtime.RawExtension{Object: payloadBar}

	g.Expect(c.Update(context.TODO(), instance)).To(Succeed())
	g.Eventually(requests, timeout, interval).Should(Receive(Equal(expectedRequest)))

	plKey = types.NamespacedName{
		Name:      payloadBar.Name,
		Namespace: deployer.Namespace,
	}
	pl = &corev1.ConfigMap{}
	g.Expect(c.Get(context.TODO(), plKey, pl)).To(Succeed())
	defer c.Delete(context.TODO(), pl)
	g.Expect(pl.Data).To(Equal(payloadBar.Data))

	//Expect payload ro be removed on hybriddeployable delete
	c.Delete(context.TODO(), instance)
	g.Eventually(requests, timeout, interval).Should(Receive(Equal(expectedRequest)))
	g.Expect(c.Get(context.TODO(), plKey, pl)).NotTo(Succeed())
}

func TestReconcileWithPlacementRule(t *testing.T) {
	g := NewWithT(t)

	templateInHybridDeployable := hybriddeployablev1alpha1.HybridTemplate{
		DeployerType: deployerType,
		Template: &runtime.RawExtension{
			Object: payloadFoo,
		},
	}

	hybridDeployable.Spec = hybriddeployablev1alpha1.HybridDeployableSpec{
		HybridTemplates: []hybriddeployablev1alpha1.HybridTemplate{
			templateInHybridDeployable,
		},
		Placement: &hybriddeployablev1alpha1.HybridPlacement{
			PlacementRef: &corev1.ObjectReference{
				Name:      placementRuleName,
				Namespace: placementRuleNamespace,
			},
		},
	}

	var c client.Client

	// Setup the Manager and Controller.  Wrap the Controller Reconcile function so it writes each request to a
	// channel when it is finished.
	mgr, err := manager.New(cfg, manager.Options{MetricsBindAddress: "0"})
	g.Expect(err).NotTo(HaveOccurred())

	c = mgr.GetClient()

	rec := newReconciler(mgr)
	recFn, requests := SetupTestReconcile(rec)

	g.Expect(add(mgr, recFn)).To(Succeed())

	stopMgr, mgrStopped := StartTestManager(mgr, g)

	defer func() {
		close(stopMgr)
		mgrStopped.Wait()
	}()

	prule := placementRule.DeepCopy()
	g.Expect(c.Create(context.TODO(), prule)).To(Succeed())
	defer c.Delete(context.TODO(), prule)

	dplyr := deployer.DeepCopy()
	g.Expect(c.Create(context.TODO(), dplyr)).To(Succeed())
	defer c.Delete(context.TODO(), dplyr)

	dset := deployerSet.DeepCopy()
	g.Expect(c.Create(context.TODO(), dset)).To(Succeed())
	defer c.Delete(context.TODO(), dset)

	clstr := cluster.DeepCopy()
	g.Expect(c.Create(context.TODO(), clstr)).To(Succeed())
	defer c.Delete(context.TODO(), clstr)

	//Pull back the placementrule and update the status subresource
	pr := &placementv1alpha1.PlacementRule{}
	g.Expect(c.Get(context.TODO(), placementRuleKey, pr)).To(Succeed())

	decisionInPlacement := placementv1alpha1.PlacementDecision{
		ClusterName:      clusterName,
		ClusterNamespace: clusterNamespace,
	}

	newpd := []placementv1alpha1.PlacementDecision{
		decisionInPlacement,
	}
	pr.Status.Decisions = newpd
	g.Expect(c.Status().Update(context.TODO(), pr.DeepCopy())).To(Succeed())
	defer c.Delete(context.TODO(), pr)

	//Expect deployable is created on hybriddeployable create
	instance := hybridDeployable.DeepCopy()
	g.Expect(c.Create(context.TODO(), instance)).To(Succeed())
	g.Eventually(requests, timeout, interval).Should(Receive(Equal(expectedRequest)))

	deployableList := &deployablev1alpha1.DeployableList{}
	g.Expect(c.List(context.TODO(), deployableList, client.InNamespace(deployerNamespace))).To(Succeed())
	g.Expect(deployableList.Items).To(HaveLen(1))
	g.Expect(deployableList.Items[0].GetGenerateName()).To(ContainSubstring(hybridDeployable.GetName() + "-" + deployerType))

	//status update reconciliation
	g.Eventually(requests, timeout, interval).Should(Receive(Equal(expectedRequest)))

	//Expect deployble is updated on hybriddeployable template update
	instance = &hybriddeployablev1alpha1.HybridDeployable{}
	g.Expect(c.Get(context.TODO(), hybridDeployableKey, instance)).To(Succeed())
	instance.Spec.HybridTemplates[0].Template = &runtime.RawExtension{Object: payloadBar}
	g.Expect(c.Update(context.TODO(), instance)).To(Succeed())
	g.Eventually(requests, timeout, interval).Should(Receive(Equal(expectedRequest)))

	deployableKey := types.NamespacedName{
		Name:      deployableList.Items[0].Name,
		Namespace: deployableList.Items[0].Namespace,
	}
	dpl := &deployablev1alpha1.Deployable{}
	g.Expect(c.Get(context.TODO(), deployableKey, dpl)).To(Succeed())
	defer c.Delete(context.TODO(), dpl)

	tpl := dpl.Spec.Template
	codecs := serializer.NewCodecFactory(mgr.GetScheme())
	tplobj, _, err := codecs.UniversalDeserializer().Decode(tpl.Raw, nil, nil)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(tplobj).To(Equal(payloadBar))

	//Expect deployable ro be removed on hybriddeployable delete
	c.Delete(context.TODO(), instance)
	g.Eventually(requests, timeout, interval).Should(Receive(Equal(expectedRequest)))
	g.Expect(c.Get(context.TODO(), deployableKey, dpl)).NotTo(Succeed())
}
