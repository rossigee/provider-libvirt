/*
Copyright 2025 Ross Golder
*/

package secret

import (
	"context"
	"encoding/base64"
	"fmt"
	"github.com/crossplane/crossplane-runtime/v2/pkg/event"
	"github.com/crossplane/crossplane-runtime/v2/pkg/logging"
	"github.com/crossplane/crossplane-runtime/v2/pkg/reconciler/managed"
	"github.com/crossplane/crossplane-runtime/v2/pkg/resource"
	xpv1 "github.com/crossplane/crossplane/apis/v2/core/v2"
	"github.com/pkg/errors"

	"github.com/rossigee/provider-libvirt/apis/v1beta1"
	"github.com/rossigee/provider-libvirt/internal/clients"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"libvirt.org/go/libvirt"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
)

const (
	errNotSecret        = "managed resource is not a Secret custom resource"
	errTrackPCUsage     = "cannot track ProviderConfig usage"
	errGetPC            = "cannot get ProviderConfig"
	errGetCreds         = "cannot get credentials"
	errNewClient        = "cannot create new libvirt client"
	errCreateSecret     = "cannot create secret"
	errDeleteSecret     = "cannot delete secret"
	errDescribeSecret   = "cannot describe secret"
	errUpdateSecret     = "cannot update secret"
	errGetKubeSecret    = "cannot get Kubernetes secret"
	errInvalidSecretRef = "invalid secret reference"
	errMissingSecretKey = "missing key in Kubernetes secret"
)

// Setup adds a controller that reconciles Secret managed resources.
func Setup(mgr ctrl.Manager, l logging.Logger) error {
	name := managed.ControllerName(v1beta1.SecretGroupKind.String())

	r := managed.NewReconciler(mgr,
		resource.ManagedKind(v1beta1.SecretGroupVersionKind),
		managed.WithExternalConnector(&connector{
			kube:         mgr.GetClient(),
			usage:        resource.TrackerFn(func(ctx context.Context, mg resource.Managed) error { return nil }),
			newServiceFn: clients.GetLibvirtClient,
		}),
		managed.WithLogger(l.WithValues("controller", name)),
		managed.WithPollInterval(clients.DefaultPollInterval),
		managed.WithRecorder(event.NewAPIRecorder(mgr.GetEventRecorder(name))))

	return ctrl.NewControllerManagedBy(mgr).
		Named(name).
		WithOptions(controller.Options{
			MaxConcurrentReconciles: clients.DefaultMaxConcurrentReconciles,
		}).
		For(&v1beta1.Secret{}).
		Complete(r)
}

// A connector is expected to produce an ExternalClient when its Connect method
// is called.
type connector struct {
	kube         client.Client
	usage        resource.Tracker
	newServiceFn func(ctx context.Context, kube client.Client, mg resource.Managed) (*clients.LibvirtClient, error)
}

// Connect typically produces an ExternalClient by:
// 1. Tracking that the managed resource is using a ProviderConfig.
// 2. Getting the managed resource's ProviderConfig.
// 3. Getting the credentials specified by the ProviderConfig.
// 4. Using the credentials to form a client.
func (c *connector) Connect(ctx context.Context, mg resource.Managed) (managed.ExternalClient, error) {
	_, ok := mg.(*v1beta1.Secret)
	if !ok {
		return nil, errors.New(errNotSecret)
	}

	if err := c.usage.Track(ctx, mg); err != nil {
		return nil, errors.Wrap(err, errTrackPCUsage)
	}

	svc, err := c.newServiceFn(ctx, c.kube, mg)
	if err != nil {
		// Connection errors are handled by GetLibvirtClient with backoff logic
		// Return as-is so managed reconciler can handle requeue
		return nil, err
	}

	return &external{
		service: svc,
		kube:    c.kube,
	}, nil
}

// An ExternalClient observes, then either creates, updates, or deletes an
// external resource to ensure it reflects the managed resource's desired state.
type external struct {
	service *clients.LibvirtClient
	kube    client.Client
}

func (c *external) Observe(ctx context.Context, mg resource.Managed) (managed.ExternalObservation, error) {
	cr, ok := mg.(*v1beta1.Secret)
	if !ok {
		return managed.ExternalObservation{}, errors.New(errNotSecret)
	}

	// If we don't have a UUID yet, the secret doesn't exist
	if cr.Status.AtProvider.UUID == "" {
		return managed.ExternalObservation{
			ResourceExists: false,
		}, nil
	}

	// Check if the secret exists in libvirt
	secret, err := c.service.SecretLookupByUUID(cr.Status.AtProvider.UUID)
	if err != nil {
		if clients.IsNotFound(err) {
			return managed.ExternalObservation{
				ResourceExists: false,
			}, nil
		}
		return managed.ExternalObservation{}, errors.Wrap(err, errDescribeSecret)
	}

	// Update observed state
	uuid, err := secret.GetUUIDString()
	if err != nil {
		return managed.ExternalObservation{}, errors.Wrap(err, "cannot get secret UUID")
	}
	cr.Status.AtProvider.UUID = uuid

	usageType, err := secret.GetUsageType()
	if err != nil {
		return managed.ExternalObservation{}, errors.Wrap(err, "cannot get secret usage type")
	}
	cr.Status.AtProvider.UsageType = fmt.Sprintf("%d", usageType)

	usageID, err := secret.GetUsageID()
	if err != nil {
		return managed.ExternalObservation{}, errors.Wrap(err, "cannot get secret usage ID")
	}
	cr.Status.AtProvider.UsageID = usageID
	cr.Status.AtProvider.Type = cr.Spec.ForProvider.Type
	cr.Status.AtProvider.Description = cr.Spec.ForProvider.Description

	// Check if secret data needs updating by comparing usage patterns
	upToDate := c.isUpToDate(cr, *secret)

	cr.Status.SetConditions(xpv1.Available())

	return managed.ExternalObservation{
		ResourceExists:   true,
		ResourceUpToDate: upToDate,
	}, nil
}

func (c *external) Create(ctx context.Context, mg resource.Managed) (managed.ExternalCreation, error) {
	cr, ok := mg.(*v1beta1.Secret)
	if !ok {
		return managed.ExternalCreation{}, errors.New(errNotSecret)
	}

	cr.Status.SetConditions(xpv1.Creating())

	// Generate secret XML based on type
	secretXML, err := c.generateSecretXML(ctx, cr)
	if err != nil {
		return managed.ExternalCreation{}, errors.Wrap(err, errCreateSecret)
	}

	// Create the secret in libvirt
	secret, err := c.service.SecretDefineXML(secretXML, 0)
	if err != nil {
		return managed.ExternalCreation{}, errors.Wrap(err, errCreateSecret)
	}

	// Set the secret value
	secretValue, err := c.getSecretValue(ctx, cr)
	if err != nil {
		return managed.ExternalCreation{}, errors.Wrap(err, errCreateSecret)
	}

	if err := c.service.SecretSetValue(secret, secretValue, 0); err != nil {
		return managed.ExternalCreation{}, errors.Wrap(err, errCreateSecret)
	}

	// Update status
	uuid, err := secret.GetUUIDString()
	if err != nil {
		return managed.ExternalCreation{}, errors.Wrap(err, "cannot get secret UUID")
	}
	cr.Status.AtProvider.UUID = uuid
	cr.Status.AtProvider.UsageType = getUsageType(cr.Spec.ForProvider.Type, cr.Spec.ForProvider.Usage)
	cr.Status.AtProvider.UsageID = generateUsageID(cr)
	cr.Status.AtProvider.Type = cr.Spec.ForProvider.Type

	return managed.ExternalCreation{}, nil
}

func (c *external) Update(ctx context.Context, mg resource.Managed) (managed.ExternalUpdate, error) {
	cr, ok := mg.(*v1beta1.Secret)
	if !ok {
		return managed.ExternalUpdate{}, errors.New(errNotSecret)
	}

	// Get the secret value and update it
	secretValue, err := c.getSecretValue(ctx, cr)
	if err != nil {
		return managed.ExternalUpdate{}, errors.Wrap(err, errUpdateSecret)
	}

	secret, err := c.service.SecretLookupByUUID(cr.Status.AtProvider.UUID)
	if err != nil {
		return managed.ExternalUpdate{}, errors.Wrap(err, "cannot find secret")
	}
	if err := c.service.SecretSetValue(secret, secretValue, 0); err != nil {
		return managed.ExternalUpdate{}, errors.Wrap(err, errUpdateSecret)
	}

	return managed.ExternalUpdate{}, nil
}

func (c *external) Delete(ctx context.Context, mg resource.Managed) (managed.ExternalDelete, error) {
	cr, ok := mg.(*v1beta1.Secret)
	if !ok {
		return managed.ExternalDelete{}, errors.New(errNotSecret)
	}

	cr.Status.SetConditions(xpv1.Deleting())

	if cr.Status.AtProvider.UUID == "" {
		return managed.ExternalDelete{}, nil
	}

	secret, err := c.service.SecretLookupByUUID(cr.Status.AtProvider.UUID)
	if err != nil {
		if clients.IsNotFound(err) {
			return managed.ExternalDelete{}, nil // Already deleted
		}
		return managed.ExternalDelete{}, errors.Wrap(err, "cannot find secret")
	}
	return managed.ExternalDelete{}, errors.Wrap(c.service.SecretUndefine(secret), errDeleteSecret)
}

func (c *external) Disconnect(ctx context.Context) error {
	// Nothing to disconnect for libvirt client
	return nil
}

// Helper functions

func (c *external) isUpToDate(cr *v1beta1.Secret, secret libvirt.Secret) bool {
	// For secrets, we primarily check if the basic metadata matches
	// Secret values are not readable from libvirt for security reasons
	expectedUsageType := getUsageType(cr.Spec.ForProvider.Type, cr.Spec.ForProvider.Usage)
	expectedUsageID := generateUsageID(cr)

	// Get actual usage type and ID from secret
	usageType, err := secret.GetUsageType()
	if err != nil {
		return false
	}
	usageID, err := secret.GetUsageID()
	if err != nil {
		return false
	}

	// Convert libvirt UsageType (int) to string for comparison
	actualUsageType := fmt.Sprintf("%d", usageType)

	return actualUsageType == expectedUsageType && usageID == expectedUsageID
}

func (c *external) generateSecretXML(ctx context.Context, cr *v1beta1.Secret) (string, error) {
	usageType := getUsageType(cr.Spec.ForProvider.Type, cr.Spec.ForProvider.Usage)
	usageID := generateUsageID(cr)

	description := cr.Spec.ForProvider.Description
	if description == "" {
		description = fmt.Sprintf("Secret for %s %s", cr.Spec.ForProvider.Type, cr.Spec.ForProvider.Usage)
	}

	ephemeral := ""
	if cr.Spec.ForProvider.Ephemeral != nil && *cr.Spec.ForProvider.Ephemeral {
		ephemeral = ` ephemeral="yes"`
	}

	private := ""
	if cr.Spec.ForProvider.Private != nil && *cr.Spec.ForProvider.Private {
		private = ` private="yes"`
	}

	xml := fmt.Sprintf(`<secret%s%s>
  <description>%s</description>
  <usage type="%s">
    <%s>%s</%s>
  </usage>
</secret>`, ephemeral, private, description, usageType, getUsageElementName(usageType), usageID, getUsageElementName(usageType))

	return xml, nil
}

func (c *external) getSecretValue(ctx context.Context, cr *v1beta1.Secret) ([]byte, error) {
	switch cr.Spec.ForProvider.Type {
	case "volume":
		return c.getVolumeSecretValue(ctx, cr)
	case "ceph":
		return c.getCephSecretValue(ctx, cr)
	case "iscsi":
		return c.getISCSISecretValue(ctx, cr)
	case "tls":
		return c.getTLSSecretValue(ctx, cr)
	default:
		return nil, fmt.Errorf("unsupported secret type: %s", cr.Spec.ForProvider.Type)
	}
}

func (c *external) getVolumeSecretValue(ctx context.Context, cr *v1beta1.Secret) ([]byte, error) {
	if cr.Spec.ForProvider.Data.Volume == nil {
		return nil, errors.New("volume secret data is required")
	}

	volumeData := cr.Spec.ForProvider.Data.Volume

	// Handle passphrase
	if volumeData.Passphrase != nil {
		return c.getSecretFromK8s(ctx, volumeData.Passphrase)
	}

	// Handle AES key
	if volumeData.AESKey != nil {
		keyData, err := c.getSecretFromK8s(ctx, volumeData.AESKey)
		if err != nil {
			return nil, err
		}
		// Ensure the key is base64 decoded if needed
		if decoded, err := base64.StdEncoding.DecodeString(string(keyData)); err == nil {
			return decoded, nil
		}
		return keyData, nil
	}

	return nil, errors.New("either passphrase or aesKey must be specified for volume secrets")
}

func (c *external) getCephSecretValue(ctx context.Context, cr *v1beta1.Secret) ([]byte, error) {
	if cr.Spec.ForProvider.Data.Ceph == nil || cr.Spec.ForProvider.Data.Ceph.Key == nil {
		return nil, errors.New("ceph key is required")
	}

	return c.getSecretFromK8s(ctx, cr.Spec.ForProvider.Data.Ceph.Key)
}

func (c *external) getISCSISecretValue(ctx context.Context, cr *v1beta1.Secret) ([]byte, error) {
	if cr.Spec.ForProvider.Data.ISCSI == nil || cr.Spec.ForProvider.Data.ISCSI.Password == nil {
		return nil, errors.New("iscsi password is required")
	}

	return c.getSecretFromK8s(ctx, cr.Spec.ForProvider.Data.ISCSI.Password)
}

func (c *external) getTLSSecretValue(ctx context.Context, cr *v1beta1.Secret) ([]byte, error) {
	if cr.Spec.ForProvider.Data.TLS == nil || cr.Spec.ForProvider.Data.TLS.Certificate == nil {
		return nil, errors.New("tls certificate is required")
	}

	return c.getSecretFromK8s(ctx, cr.Spec.ForProvider.Data.TLS.Certificate)
}

func (c *external) getSecretFromK8s(ctx context.Context, ref *v1beta1.SecretReference) ([]byte, error) {
	if ref.Name == "" || ref.Key == "" {
		return nil, errors.New(errInvalidSecretRef)
	}

	namespace := ref.Namespace
	if namespace == "" {
		namespace = "default"
	}

	secret := &corev1.Secret{}
	if err := c.kube.Get(ctx, types.NamespacedName{
		Name:      ref.Name,
		Namespace: namespace,
	}, secret); err != nil {
		return nil, errors.Wrap(err, errGetKubeSecret)
	}

	data, exists := secret.Data[ref.Key]
	if !exists {
		return nil, errors.New(errMissingSecretKey)
	}

	return data, nil
}

// Utility functions

func getUsageType(secretType, usage string) string {
	// Return the libvirt usage type based on secret type
	// These correspond to libvirt's secret usage types
	switch secretType {
	case "volume":
		return "volume"
	case "ceph":
		return "ceph"
	case "iscsi":
		return "iscsi"
	case "tls":
		return "tls"
	default:
		return "volume"
	}
}

func generateUsageID(cr *v1beta1.Secret) string {
	// Generate a usage ID based on the secret name and namespace
	usageID := cr.Name
	if cr.Namespace != "" {
		usageID = fmt.Sprintf("%s-%s", cr.Namespace, cr.Name)
	}
	return usageID
}

func getUsageElementName(usageType string) string {
	switch usageType {
	case "volume":
		return "volume"
	case "ceph":
		return "name"
	case "iscsi":
		return "target"
	case "tls":
		return "name"
	default:
		return "volume"
	}
}
