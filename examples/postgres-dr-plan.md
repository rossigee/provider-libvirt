# PostgreSQL DR VM Specifications

## Infrastructure Overview
- **Target Host**: timewarp-001.timewarp.lan:16514
- **Provider**: libvirt v0.3.5 (deployed and healthy)
- **Network**: postgres-dr-network (192.168.100.0/24)
- **Storage**: Local libvirt storage pools

## VM Specifications

### Primary PostgreSQL Server
- **Name**: postgres-dr-primary
- **CPU**: 4 vCPUs
- **Memory**: 8GB RAM
- **Storage**:
  - Root disk: 20GB (OS)
  - Data disk: 100GB (PostgreSQL data)
- **Network**: postgres-dr-network (192.168.100.10/24)
- **OS**: Ubuntu 24.04 LTS

### Secondary PostgreSQL Server (DR)
- **Name**: postgres-dr-secondary
- **CPU**: 4 vCPUs
- **Memory**: 8GB RAM
- **Storage**:
  - Root disk: 20GB (OS)
  - Data disk: 100GB (PostgreSQL data)
- **Network**: postgres-dr-network (192.168.100.11/24)
- **OS**: Ubuntu 24.04 LTS

## Storage Pool Configuration
- **Pool Name**: postgres-dr-pool
- **Type**: dir
- **Path**: /var/lib/libvirt/images/postgres-dr
- **Capacity**: 300GB allocated

## Network Configuration
- **Network**: postgres-dr-network (already created)
- **DHCP Range**: 192.168.100.100-200
- **Gateway**: 192.168.100.1
- **DNS**: 8.8.8.8
- **Bridge**: virbr3

## Volume Configuration
Each VM requires:
1. Root volume (qcow2, 20GB)
2. Data volume (qcow2, 100GB)

## Resource Summary
- **Total VMs**: 2
- **Total CPU**: 8 vCPUs
- **Total Memory**: 16GB
- **Total Storage**: 240GB (4 volumes × 60GB average)