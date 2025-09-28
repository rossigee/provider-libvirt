# Storage Management Enhancements Design

## Overview
Enhanced storage management capabilities for provider-libvirt, adding enterprise-grade features like snapshots, live migration, and advanced storage backends.

## Current Storage Resources Enhancement

### 1. **Volume Snapshots** 📸
Add snapshot capabilities to existing Volume resource.

```go
// VolumeSnapshotSpec defines snapshot configuration
type VolumeSnapshotSpec struct {
    xpv1.ResourceSpec `json:",inline"`
    ForProvider       VolumeSnapshotParameters `json:"forProvider"`
}

type VolumeSnapshotParameters struct {
    // Source volume reference
    // +kubebuilder:validation:Required
    VolumeRef *xpv1.Reference `json:"volumeRef"`

    // Snapshot name
    // +kubebuilder:validation:Required
    Name string `json:"name"`

    // Snapshot description
    // +kubebuilder:validation:Optional
    Description string `json:"description,omitempty"`

    // Create snapshot atomically (CoW)
    // +kubebuilder:validation:Optional
    // +kubebuilder:default=true
    Atomic bool `json:"atomic,omitempty"`

    // Snapshot metadata
    // +kubebuilder:validation:Optional
    Metadata map[string]string `json:"metadata,omitempty"`
}

type VolumeSnapshotObservation struct {
    // Snapshot XML definition
    XML string `json:"xml,omitempty"`

    // Snapshot state
    State string `json:"state,omitempty"`

    // Creation timestamp
    CreationTime *metav1.Time `json:"creationTime,omitempty"`

    // Snapshot size
    Size int64 `json:"size,omitempty"`

    // Parent volume information
    Parent VolumeSnapshotParent `json:"parent,omitempty"`
}

type VolumeSnapshotParent struct {
    // Parent volume name
    Volume string `json:"volume"`

    // Parent volume size at snapshot time
    Size int64 `json:"size"`
}
```

### 2. **Domain Snapshots** 🖥️
Add VM state snapshot capabilities.

```go
// DomainSnapshotSpec defines VM snapshot configuration
type DomainSnapshotSpec struct {
    xpv1.ResourceSpec `json:",inline"`
    ForProvider       DomainSnapshotParameters `json:"forProvider"`
}

type DomainSnapshotParameters struct {
    // Source domain reference
    // +kubebuilder:validation:Required
    DomainRef *xpv1.Reference `json:"domainRef"`

    // Snapshot name
    // +kubebuilder:validation:Required
    Name string `json:"name"`

    // Snapshot description
    // +kubebuilder:validation:Optional
    Description string `json:"description,omitempty"`

    // Snapshot type: internal, external, disk-only
    // +kubebuilder:validation:Optional
    // +kubebuilder:validation:Enum=internal;external;disk-only
    // +kubebuilder:default=internal
    Type string `json:"type,omitempty"`

    // Include memory state
    // +kubebuilder:validation:Optional
    // +kubebuilder:default=true
    Memory bool `json:"memory,omitempty"`

    // Live snapshot (don't pause domain)
    // +kubebuilder:validation:Optional
    // +kubebuilder:default=false
    Live bool `json:"live,omitempty"`

    // Disk snapshot configuration
    // +kubebuilder:validation:Optional
    Disks []DomainSnapshotDisk `json:"disks,omitempty"`
}

type DomainSnapshotDisk struct {
    // Target disk device name
    Name string `json:"name"`

    // Snapshot type for this disk
    // +kubebuilder:validation:Enum=internal;external;no
    Type string `json:"type"`

    // External file path (for external snapshots)
    // +kubebuilder:validation:Optional
    File string `json:"file,omitempty"`
}

type DomainSnapshotObservation struct {
    // Snapshot state
    State string `json:"state,omitempty"`

    // Domain state at snapshot time
    DomainState string `json:"domainState,omitempty"`

    // Creation timestamp
    CreationTime *metav1.Time `json:"creationTime,omitempty"`

    // Snapshot tree information
    Parent string `json:"parent,omitempty"`
    Children []string `json:"children,omitempty"`

    // Memory file location (for external memory snapshots)
    MemoryFile string `json:"memoryFile,omitempty"`

    // Disk snapshot details
    DisksInfo []DomainSnapshotDiskInfo `json:"disksInfo,omitempty"`
}

type DomainSnapshotDiskInfo struct {
    // Disk name
    Name string `json:"name"`

    // Snapshot file
    File string `json:"file"`

    // Snapshot size
    Size int64 `json:"size"`

    // Format (qcow2, raw, etc.)
    Format string `json:"format"`
}
```

### 3. **Storage Migration** 🔄
Add live storage migration capabilities.

```go
// StorageMigrationSpec defines storage migration job
type StorageMigrationSpec struct {
    xpv1.ResourceSpec `json:",inline"`
    ForProvider       StorageMigrationParameters `json:"forProvider"`
}

type StorageMigrationParameters struct {
    // Source volume reference
    // +kubebuilder:validation:Required
    SourceVolumeRef *xpv1.Reference `json:"sourceVolumeRef"`

    // Destination storage pool reference
    // +kubebuilder:validation:Required
    DestinationPoolRef *xpv1.Reference `json:"destinationPoolRef"`

    // Destination volume name (optional, defaults to source name)
    // +kubebuilder:validation:Optional
    DestinationName string `json:"destinationName,omitempty"`

    // Migration bandwidth limit (MB/s)
    // +kubebuilder:validation:Optional
    // +kubebuilder:default=0
    BandwidthLimit int64 `json:"bandwidthLimit,omitempty"`

    // Copy type: full, incremental, shared
    // +kubebuilder:validation:Optional
    // +kubebuilder:validation:Enum=full;incremental;shared
    // +kubebuilder:default=full
    CopyType string `json:"copyType,omitempty"`

    // Preserve source volume after migration
    // +kubebuilder:validation:Optional
    // +kubebuilder:default=false
    PreserveSource bool `json:"preserveSource,omitempty"`

    // Live migration (for attached volumes)
    // +kubebuilder:validation:Optional
    // +kubebuilder:default=false
    Live bool `json:"live,omitempty"`
}

type StorageMigrationObservation struct {
    // Migration job state
    State string `json:"state,omitempty"`

    // Migration progress (0-100)
    Progress int32 `json:"progress,omitempty"`

    // Data processed (bytes)
    DataProcessed int64 `json:"dataProcessed,omitempty"`

    // Data remaining (bytes)
    DataRemaining int64 `json:"dataRemaining,omitempty"`

    // Current bandwidth (MB/s)
    Bandwidth int64 `json:"bandwidth,omitempty"`

    // Migration start time
    StartTime *metav1.Time `json:"startTime,omitempty"`

    // Estimated completion time
    ETA *metav1.Time `json:"eta,omitempty"`

    // Destination volume details
    Destination StorageMigrationDestination `json:"destination,omitempty"`
}

type StorageMigrationDestination struct {
    // Destination volume name
    Name string `json:"name"`

    // Destination pool
    Pool string `json:"pool"`

    // Destination path
    Path string `json:"path"`
}
```

### 4. **Enhanced StoragePool Types** 🏗️
Extend existing StoragePool with advanced backends.

```go
// Add to existing StoragePoolParameters
type StoragePoolParameters struct {
    // ... existing fields ...

    // Advanced iSCSI configuration
    // +kubebuilder:validation:Optional
    ISCSIAdvanced *ISCSIAdvancedConfig `json:"iscsiAdvanced,omitempty"`

    // Ceph RBD configuration
    // +kubebuilder:validation:Optional
    CephRBD *CephRBDConfig `json:"cephRbd,omitempty"`

    // GlusterFS configuration
    // +kubebuilder:validation:Optional
    GlusterFS *GlusterFSConfig `json:"glusterfs,omitempty"`

    // ZFS configuration
    // +kubebuilder:validation:Optional
    ZFS *ZFSConfig `json:"zfs,omitempty"`
}

// ISCSIAdvancedConfig provides comprehensive iSCSI support
type ISCSIAdvancedConfig struct {
    // iSCSI target portal
    // +kubebuilder:validation:Required
    Portal string `json:"portal"`

    // Target IQN
    // +kubebuilder:validation:Required
    IQN string `json:"iqn"`

    // CHAP authentication
    // +kubebuilder:validation:Optional
    Auth *ISCSIAuth `json:"auth,omitempty"`

    // Initiator IQN
    // +kubebuilder:validation:Optional
    InitiatorIQN string `json:"initiatorIqn,omitempty"`

    // LUN scanning
    // +kubebuilder:validation:Optional
    // +kubebuilder:default=true
    AutoScan bool `json:"autoScan,omitempty"`
}

type ISCSIAuth struct {
    // Authentication type: chap, none
    // +kubebuilder:validation:Required
    // +kubebuilder:validation:Enum=chap;none
    Type string `json:"type"`

    // CHAP username
    // +kubebuilder:validation:Optional
    Username string `json:"username,omitempty"`

    // Secret reference for CHAP password
    // +kubebuilder:validation:Optional
    SecretRef *xpv1.Reference `json:"secretRef,omitempty"`
}

// CephRBDConfig provides Ceph RBD integration
type CephRBDConfig struct {
    // Ceph monitors
    // +kubebuilder:validation:Required
    Monitors []string `json:"monitors"`

    // RBD pool name
    // +kubebuilder:validation:Required
    Pool string `json:"pool"`

    // Ceph username
    // +kubebuilder:validation:Optional
    Username string `json:"username,omitempty"`

    // Secret reference for Ceph keyring
    // +kubebuilder:validation:Optional
    SecretRef *xpv1.Reference `json:"secretRef,omitempty"`

    // RBD namespace
    // +kubebuilder:validation:Optional
    Namespace string `json:"namespace,omitempty"`
}

// GlusterFSConfig provides GlusterFS integration
type GlusterFSConfig struct {
    // GlusterFS servers
    // +kubebuilder:validation:Required
    Servers []string `json:"servers"`

    // Volume name
    // +kubebuilder:validation:Required
    Volume string `json:"volume"`

    // Transport type: tcp, rdma
    // +kubebuilder:validation:Optional
    // +kubebuilder:validation:Enum=tcp;rdma
    // +kubebuilder:default=tcp
    Transport string `json:"transport,omitempty"`
}

// ZFSConfig provides ZFS integration
type ZFSConfig struct {
    // ZFS pool name
    // +kubebuilder:validation:Required
    Pool string `json:"pool"`

    // Dataset prefix
    // +kubebuilder:validation:Optional
    Dataset string `json:"dataset,omitempty"`
}
```

## Usage Examples

### Volume Snapshots
```yaml
apiVersion: libvirt.m.crossplane.io/v1beta1
kind: VolumeSnapshot
metadata:
  name: vm-data-backup-20250114
spec:
  forProvider:
    volumeRef:
      name: vm-data-volume
    name: backup-20250114
    description: "Daily backup before system update"
    atomic: true
    metadata:
      backup-policy: "daily"
      retention-days: "30"
```

### Domain Snapshots
```yaml
apiVersion: libvirt.m.crossplane.io/v1beta1
kind: DomainSnapshot
metadata:
  name: web-server-pre-update
spec:
  forProvider:
    domainRef:
      name: web-server
    name: pre-update-snapshot
    description: "Before critical security updates"
    type: external
    memory: true
    live: true
    disks:
    - name: vda
      type: external
      file: /var/lib/libvirt/snapshots/web-server-vda-snap.qcow2
```

### Storage Migration
```yaml
apiVersion: libvirt.m.crossplane.io/v1beta1
kind: StorageMigration
metadata:
  name: migrate-to-ssd-pool
spec:
  forProvider:
    sourceVolumeRef:
      name: database-volume
    destinationPoolRef:
      name: ssd-pool
    copyType: full
    bandwidthLimit: 1000  # 1GB/s
    live: true
    preserveSource: false
```

### Advanced Storage Pools
```yaml
# Ceph RBD Pool
apiVersion: libvirt.m.crossplane.io/v1beta1
kind: StoragePool
metadata:
  name: ceph-rbd-pool
spec:
  forProvider:
    name: ceph-rbd
    type: rbd
    cephRbd:
      monitors: ["10.0.1.100:6789", "10.0.1.101:6789", "10.0.1.102:6789"]
      pool: libvirt-pool
      username: libvirt
      secretRef:
        name: ceph-secret
---
# High-Performance iSCSI Pool
apiVersion: libvirt.m.crossplane.io/v1beta1
kind: StoragePool
metadata:
  name: enterprise-iscsi
spec:
  forProvider:
    name: iscsi-san
    type: iscsi
    iscsiAdvanced:
      portal: "10.0.2.100:3260"
      iqn: "iqn.2025-01.com.enterprise:storage"
      auth:
        type: chap
        username: libvirt-user
        secretRef:
          name: iscsi-auth-secret
      autoScan: true
```

## Implementation Priority

### Phase 1: Foundation (3-4 weeks)
1. **Volume Snapshots**: Basic CoW snapshot support
2. **Enhanced StoragePool**: iSCSI and Ceph RBD backends

### Phase 2: Advanced Features (4-6 weeks)
3. **Domain Snapshots**: VM state snapshots
4. **Storage Migration**: Basic volume migration

### Phase 3: Enterprise Features (6-8 weeks)
5. **Live Migration**: Hot storage migration
6. **Advanced Backends**: GlusterFS, ZFS support
7. **Snapshot Management**: Snapshot chains, cleanup policies

## Enterprise Value

- **📊 Data Protection**: Comprehensive snapshot and backup capabilities
- **🔄 Flexibility**: Live migration for zero-downtime maintenance
- **🏗️ Scalability**: Support for enterprise storage backends (Ceph, iSCSI, GlusterFS)
- **⚡ Performance**: High-performance storage with bandwidth controls
- **🛡️ Reliability**: Advanced storage replication and redundancy
- **📈 Monitoring**: Migration progress and storage health visibility