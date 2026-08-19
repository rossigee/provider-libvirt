package domain

import (
	"context"

	xpv1 "github.com/crossplane/crossplane/apis/v2/core/v2"
	"github.com/pkg/errors"

	"github.com/rossigee/provider-libvirt/apis/v1beta1"
	"k8s.io/apimachinery/pkg/types"
)

// resolveVolumeRef resolves a Volume reference to get the volume path
// Returns the volume path if successful, empty string otherwise
func (c *external) resolveVolumeRef(ctx context.Context, ref *xpv1.Reference, namespace string) (string, error) {
	if ref == nil || c.kube == nil {
		return "", nil
	}

	// Construct the name from reference
	volumeName := ref.Name
	if volumeName == "" {
		return "", errors.New("volume reference has no name specified")
	}

	// Fetch the Volume resource
	vol := &v1beta1.Volume{}
	key := types.NamespacedName{
		Name:      volumeName,
		Namespace: namespace,
	}

	if err := c.kube.Get(ctx, key, vol); err != nil {
		return "", errors.Wrapf(err, "cannot get volume reference %s/%s", namespace, volumeName)
	}

	// Extract path from volume status
	if vol.Status.AtProvider.Path == "" {
		return "", errors.New("volume reference has no path in status")
	}

	return vol.Status.AtProvider.Path, nil
}

// resolveNetworkRef resolves a Network reference to get the network name
// Returns the network name if successful, empty string otherwise
func (c *external) resolveNetworkRef(ctx context.Context, ref *xpv1.Reference, namespace string) (string, error) {
	if ref == nil || c.kube == nil {
		return "", nil
	}

	// Construct the name from reference
	networkName := ref.Name
	if networkName == "" {
		return "", errors.New("network reference has no name specified")
	}

	// Fetch the Network resource
	net := &v1beta1.Network{}
	key := types.NamespacedName{
		Name:      networkName,
		Namespace: namespace,
	}

	if err := c.kube.Get(ctx, key, net); err != nil {
		return "", errors.Wrapf(err, "cannot get network reference %s/%s", namespace, networkName)
	}

	// Use the network's spec name as the libvirt network name
	if net.Spec.ForProvider.Name == "" {
		return "", errors.New("network reference has no name in spec")
	}

	return net.Spec.ForProvider.Name, nil
}

// resolveDiskSource resolves the disk source from either File or VolumeRef
// Prefers VolumeRef if available, falls back to File
func (c *external) resolveDiskSource(ctx context.Context, disk v1beta1.DomainDisk, namespace string) (string, error) {
	// Try to resolve VolumeRef first (higher priority)
	if disk.VolumeRef != nil {
		path, err := c.resolveVolumeRef(ctx, disk.VolumeRef, namespace)
		if err != nil {
			return "", err
		}
		if path != "" {
			return path, nil
		}
	}

	// Fall back to direct File path
	if disk.File != "" {
		return disk.File, nil
	}

	return "", errors.New("disk has neither VolumeRef nor File specified")
}

// resolveNetworkSource resolves the network source from either NetworkRef or NetworkName
// Prefers NetworkRef if available, falls back to NetworkName
func (c *external) resolveNetworkSource(ctx context.Context, ni v1beta1.DomainNetworkInterface, namespace string) (string, error) {
	// Try to resolve NetworkRef first (higher priority)
	if ni.NetworkRef != nil {
		name, err := c.resolveNetworkRef(ctx, ni.NetworkRef, namespace)
		if err != nil {
			return "", err
		}
		if name != "" {
			return name, nil
		}
	}

	// Fall back to direct NetworkName
	if ni.NetworkName != "" {
		return ni.NetworkName, nil
	}

	return "", errors.New("network interface has neither NetworkRef nor NetworkName specified")
}
