# Provider Libvirt

[![CI](https://img.shields.io/github/actions/workflow/status/rossigee/provider-libvirt/ci.yml?branch=master)][build]
[![Version](https://img.shields.io/github/v/release/rossigee/provider-libvirt)][releases]
[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](https://opensource.org/licenses/Apache-2.0)

[build]: https://github.com/rossigee/provider-libvirt/actions/workflows/ci.yml
[releases]: https://github.com/rossigee/provider-libvirt/releases

`provider-libvirt` is a [Crossplane](https://crossplane.io/) provider for managing Libvirt/KVM virtual machines.

## Container Registry

- **Primary**: `ghcr.io/rossigee/provider-libvirt:v0.2.0`

## Overview

A Crossplane provider for managing Libvirt/KVM virtual machines.

## Features

- **Virtual Machine Management**: Create, configure, and manage KVM virtual machines
- **Storage Management**: Virtual disk and storage pool management
- **Network Management**: Virtual network configuration
- **Lifecycle Operations**: Start, stop, pause, and delete VMs

## Getting Started

Install the provider:

```
kubectl crossplane install provider ghcr.io/rossigee/provider-libvirt:v0.2.0
```

Alternatively, you can use declarative installation:

```yaml
apiVersion: pkg.crossplane.io/v1
kind: Provider
metadata:
  name: provider-libvirt
spec:
  package: ghcr.io/rossigee/provider-libvirt:v0.2.0
```

You can see the API reference [here](https://doc.crds.dev/github.com/nourspeed/provider-libvirt).

## Developing

Run code-generation pipeline:
```console
go run cmd/generator/main.go "$PWD"
```

Run against a Kubernetes cluster:

```console
make run
```

Build, push, and install:

```console
make all
```

Build binary:

```console
make build
```

## Report a Bug

For filing bugs, suggesting improvements, or requesting new features, please
open an [issue](https://github.com/nourspeed/provider-libvirt/issues).
