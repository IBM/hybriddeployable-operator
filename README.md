# HybridDeployable Operator

[![Build](http://prow.purple-chesterfield.com/badge.svg?jobs=images-hybriddeployable-operator-amd64-postsubmit)](http://http://prow.purple-chesterfield.com/?job=images-hybriddeployable-operator-amd64-postsubmit)
[![GoDoc](https://godoc.org/github.com/IBM/deployer-operator?status.svg)](https://godoc.org/github.com/IBM/deployer-operator)
[![Go Report Card](https://goreportcard.com/badge/github.com/IBM/hybriddeployable-operator)](https://goreportcard.com/report/github.com/IBM/hybriddeployable-operator)
[![Code Coverage](https://codecov.io/gh/IBM/hybriddeployable-operator/branch/master/graphs/badge.svg?branch=master)](https://codecov.io/gh/IBM/hybriddeployable-operator?branch=master)
[![License](https://img.shields.io/:license-apache-blue.svg)](http://www.apache.org/licenses/LICENSE-2.0.html)
[![Container](https://quay.io/repository/multicloudlab/hybriddeployable-operator/status)](https://quay.io/repository/multicloudlab/hybriddeployable-operator?tab=tags)

<!-- START doctoc generated TOC please keep comment here to allow auto update -->
<!-- DON'T EDIT THIS SECTION, INSTEAD RE-RUN doctoc TO UPDATE -->
**Table of Contents**  *generated with [DocToc](https://github.com/thlorenz/doctoc)*

- [What is the Hybrid Deployable Operator](#what-is-the-hybrid-deployable-operator)
- [Community, discussion, contribution, and support](#community-discussion-contribution-and-support)
- [Getting Started](#getting-started)
    - [Prerequisites](#prerequisites)
    - [Quick Start](#quick-start)
    - [Trouble shooting](#trouble-shooting)
- [Hybrid Application References](#hybrid-application-references)

<!-- END doctoc generated TOC please keep comment here to allow auto update -->

## What is the HybridDeployable Operator

The hybridDeployable resource is introduced to handle deployable components running on non-kubernetes platform(s). This operator is intended to work as part of collection of operators for the HybridApplication.  See [References](#hybridApplication-references) for additional information.

## Community, discussion, contribution, and support

Check the [CONTRIBUTING Doc](CONTRIBUTING.md) for how to contribute to the repo.

------

## Getting Started

### Prerequisites

- git v2.18+
- Go v1.13.4+
- operator-sdk v0.15.1
- Kubernetes v1.14+
- kubectl v1.14+

Check the [Development Doc](docs/development.md) for how to contribute to the repo.

### Quick Start

#### Clone HybridDeployable Operator Repository

```shell
$ mkdir -p "$GOPATH"/src/github.com/IBM
$ cd "$GOPATH"/src/github.com/IBM
$ git clone https://github.com/IBM/hybriddeployable-operator.git
$ cd "$GOPATH"/src/github.com/IBM/hybriddeployable-operator
```

#### Build HybridDeployable Operator

Build the hybriddeployable-operator and push it to a registry.  Modify the example below to reference a container reposistory you have access to.

```shell
$ operator-sdk build quay.io/<user>/hybriddeployable-operator:v0.1.0
$ sed -i 's|REPLACE_IMAGE|quay.io/johndoe/hybriddeployable-operator:v0.1.0|g' deploy/operator.yaml
$ docker push quay.io/johndoe/hybriddeployable-operator:v0.1.0
```

#### Install HybridDeployable Operator

Register the CRD.

```shell
$ kubectl create -f deploy/crds/app.cp4mcm.ibm.com_hybriddeployables_crd.yaml
```

Setup RBAC and deploy.

```shell
$ kubectl create -f deploy/service_account.yaml
$ kubectl create -f deploy/role.yaml
$ kubectl create -f deploy/role_binding.yaml
$ kubectl create -f deploy/operator.yaml
```

Verify hybriddeployable-operator is up and running.

```shell
$ kubectl get deployment
NAME                        READY   UP-TO-DATE   AVAILABLE   AGE
hybriddeployable-operator   1/1     1            1           2m20s
```

Create the sample CR.

```shell
$ kubectl create -f deploy/crds/app.cp4mcm.ibm.com_hybriddeployables_cr.yaml
NAME                        READY   UP-TO-DATE   AVAILABLE   AGE
hybriddeployable-operator   1/1     1            1           2m20s
$ kubectl get hybriddeployables
NAME     AGE
simple   11s
```

#### Uninstall HybridDeployable Operator

Remove all resources created.

```shell
$ kubectl delete -f deploy
$ kubectl delete -f deploy/crds/app.cp4mcm.ibm.com_hybriddeployables_crd.yaml
```

### Troubleshooting

Please refer to [Troubleshooting documentation](docs/trouble_shooting.md) for further info.

## References

### HybridApplication Related Repositories

- [HybridApplication Operater](https://github.com/IBM/hybridapplication-operator)
- [HybridDeployable Operator](https://github.com/IBM/hybriddeployable-operator)
- [Deployer Operator](https://github.com/IBM/deployer-operator)
