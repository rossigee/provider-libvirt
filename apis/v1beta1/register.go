/*
Copyright 2025 Ross Golder
*/

package v1beta1

import (
	"k8s.io/apimachinery/pkg/runtime"
)

// Package-wide variables whose init functions are called in the init() function
// of the containing package's parent `init.go` file.
var (
	// SchemeGroupVersion is group version used to register these objects
	SchemeGroupVersion = schema.GroupVersion{Group: Group, Version: Version}

	// SchemeBuilder is used to add go types to the GroupVersionKind scheme
	//nolint:staticcheck // SA1019: scheme.Builder is deprecated but required for Crossplane API pattern
	SchemeBuilder = runtime.NewSchemeBuilder(addKnownTypes)
	AddToScheme   = SchemeBuilder.AddToScheme
)

const (
	Group   = "libvirt.m.crossplane.io"
	Version = "v1beta1"
)

func addKnownTypes(s *runtime.Scheme) error {
	s.AddKnownTypes(SchemeGroupVersion,
		&ProviderConfigUsage{},
		&ProviderConfigUsageList{},
		&Domain{},
		&DomainList{},
		&Secret{},
		&SecretList{},
		&Network{},
		&NetworkList{},
		&ProviderConfig{},
		&ProviderConfigList{},
		&Volume{},
		&VolumeList{},
		&NodeDevice{},
		&NodeDeviceList{},
		&StoragePool{},
		&StoragePoolList{},
	)
	return nil
}
