package secret

import (
	"context"
	"github.com/rossigee/provider-libvirt/apis/v1beta1"
	"k8s.io/apimachinery/pkg/apis/meta/v1"
	"testing"
)

// Helper function tests for secret controller

func TestGenerateSecretXMLBasic(t *testing.T) {
	ext := &external{service: nil, kube: nil}
	cr := &v1beta1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-secret",
		},
		Spec: v1beta1.SecretSpec{
			ForProvider: v1beta1.SecretParameters{
				Type:        "volume",
				Usage:       "test-vol",
				Description: "Test secret",
			},
		},
	}

	xml, err := ext.generateSecretXML(context.Background(), cr)
	if err != nil {
		t.Errorf("generateSecretXML failed: %v", err)
	}
	if xml == "" {
		t.Error("generateSecretXML should return non-empty XML")
	}
	if !contains(xml, "Test secret") {
		t.Error("XML should contain description")
	}
	if !contains(xml, "volume") {
		t.Error("XML should contain secret type")
	}
}

func TestGenerateSecretXMLEmptyDescription(t *testing.T) {
	ext := &external{service: nil, kube: nil}
	cr := &v1beta1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-secret",
		},
		Spec: v1beta1.SecretSpec{
			ForProvider: v1beta1.SecretParameters{
				Type:  "volume",
				Usage: "test-vol",
			},
		},
	}

	xml, err := ext.generateSecretXML(context.Background(), cr)
	if err != nil {
		t.Errorf("generateSecretXML failed: %v", err)
	}
	if xml == "" {
		t.Error("Should generate default description when empty")
	}
	if !contains(xml, "Secret for volume test-vol") {
		t.Error("Should include generated description in XML")
	}
}

func TestGenerateSecretXMLEphemeral(t *testing.T) {
	ext := &external{service: nil, kube: nil}
	t1 := true
	cr := &v1beta1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-secret",
		},
		Spec: v1beta1.SecretSpec{
			ForProvider: v1beta1.SecretParameters{
				Type:      "volume",
				Usage:     "test-vol",
				Ephemeral: &t1,
			},
		},
	}

	xml, err := ext.generateSecretXML(context.Background(), cr)
	if err != nil {
		t.Errorf("generateSecretXML failed: %v", err)
	}
	if !contains(xml, `ephemeral="yes"`) {
		t.Error("XML should have ephemeral attribute")
	}
}

func TestGenerateSecretXMLPrivate(t *testing.T) {
	ext := &external{service: nil, kube: nil}
	t1 := true
	cr := &v1beta1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-secret",
		},
		Spec: v1beta1.SecretSpec{
			ForProvider: v1beta1.SecretParameters{
				Type:    "volume",
				Usage:   "test-vol",
				Private: &t1,
			},
		},
	}

	xml, err := ext.generateSecretXML(context.Background(), cr)
	if err != nil {
		t.Errorf("generateSecretXML failed: %v", err)
	}
	if !contains(xml, `private="yes"`) {
		t.Error("XML should have private attribute")
	}
}

func TestGenerateSecretXMLCephType(t *testing.T) {
	ext := &external{service: nil, kube: nil}
	cr := &v1beta1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-secret",
		},
		Spec: v1beta1.SecretSpec{
			ForProvider: v1beta1.SecretParameters{
				Type:  "ceph",
				Usage: "ceph-user",
			},
		},
	}

	xml, err := ext.generateSecretXML(context.Background(), cr)
	if err != nil {
		t.Errorf("generateSecretXML failed: %v", err)
	}
	if !contains(xml, "ceph") {
		t.Error("XML should contain ceph type")
	}
}

func TestGenerateSecretXMLISCSIType(t *testing.T) {
	ext := &external{service: nil, kube: nil}
	cr := &v1beta1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-secret",
		},
		Spec: v1beta1.SecretSpec{
			ForProvider: v1beta1.SecretParameters{
				Type:  "iscsi",
				Usage: "iscsi-user",
			},
		},
	}

	xml, err := ext.generateSecretXML(context.Background(), cr)
	if err != nil {
		t.Errorf("generateSecretXML failed: %v", err)
	}
	if !contains(xml, "iscsi") {
		t.Error("XML should contain iscsi type")
	}
}

func TestGenerateSecretXMLTLSType(t *testing.T) {
	ext := &external{service: nil, kube: nil}
	cr := &v1beta1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-secret",
		},
		Spec: v1beta1.SecretSpec{
			ForProvider: v1beta1.SecretParameters{
				Type:  "tls",
				Usage: "server-cert",
			},
		},
	}

	xml, err := ext.generateSecretXML(context.Background(), cr)
	if err != nil {
		t.Errorf("generateSecretXML failed: %v", err)
	}
	if !contains(xml, "tls") {
		t.Error("XML should contain tls type")
	}
}

func TestGetSecretValueUnsupportedType(t *testing.T) {
	ext := &external{service: nil, kube: nil}
	cr := &v1beta1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-secret",
		},
		Spec: v1beta1.SecretSpec{
			ForProvider: v1beta1.SecretParameters{
				Type:  "invalid-type",
				Usage: "test",
			},
		},
	}

	_, err := ext.getSecretValue(context.Background(), cr)
	if err == nil {
		t.Error("getSecretValue should fail for unsupported type")
	}
	if !contains(err.Error(), "unsupported secret type") {
		t.Errorf("Expected 'unsupported secret type' error, got: %v", err)
	}
}

func TestGetVolumeSecretValueMissingData(t *testing.T) {
	ext := &external{service: nil, kube: nil}
	cr := &v1beta1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-secret",
		},
		Spec: v1beta1.SecretSpec{
			ForProvider: v1beta1.SecretParameters{
				Type:  "volume",
				Usage: "test-vol",
				Data: v1beta1.SecretData{
					Volume: nil,
				},
			},
		},
	}

	_, err := ext.getVolumeSecretValue(context.Background(), cr)
	if err == nil {
		t.Error("getVolumeSecretValue should fail when data is nil")
	}
	if !contains(err.Error(), "volume secret data is required") {
		t.Errorf("Expected 'volume secret data is required' error, got: %v", err)
	}
}

func TestGetCephSecretValueMissingData(t *testing.T) {
	ext := &external{service: nil, kube: nil}
	cr := &v1beta1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-secret",
		},
		Spec: v1beta1.SecretSpec{
			ForProvider: v1beta1.SecretParameters{
				Type:  "ceph",
				Usage: "ceph-user",
				Data: v1beta1.SecretData{
					Ceph: nil,
				},
			},
		},
	}

	_, err := ext.getCephSecretValue(context.Background(), cr)
	if err == nil {
		t.Error("getCephSecretValue should fail when data is nil")
	}
	if !contains(err.Error(), "ceph key is required") {
		t.Errorf("Expected 'ceph key is required' error, got: %v", err)
	}
}

func TestGetISCSISecretValueMissingData(t *testing.T) {
	ext := &external{service: nil, kube: nil}
	cr := &v1beta1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-secret",
		},
		Spec: v1beta1.SecretSpec{
			ForProvider: v1beta1.SecretParameters{
				Type:  "iscsi",
				Usage: "iscsi-user",
				Data: v1beta1.SecretData{
					ISCSI: nil,
				},
			},
		},
	}

	_, err := ext.getISCSISecretValue(context.Background(), cr)
	if err == nil {
		t.Error("getISCSISecretValue should fail when data is nil")
	}
	if !contains(err.Error(), "iscsi password is required") {
		t.Errorf("Expected 'iscsi password is required' error, got: %v", err)
	}
}

func TestGetTLSSecretValueMissingData(t *testing.T) {
	ext := &external{service: nil, kube: nil}
	cr := &v1beta1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-secret",
		},
		Spec: v1beta1.SecretSpec{
			ForProvider: v1beta1.SecretParameters{
				Type:  "tls",
				Usage: "server-cert",
				Data: v1beta1.SecretData{
					TLS: nil,
				},
			},
		},
	}

	_, err := ext.getTLSSecretValue(context.Background(), cr)
	if err == nil {
		t.Error("getTLSSecretValue should fail when data is nil")
	}
	if !contains(err.Error(), "tls certificate is required") {
		t.Errorf("Expected 'tls certificate is required' error, got: %v", err)
	}
}

func TestGenerateUsageIDConsistency(t *testing.T) {
	cr := &v1beta1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-secret",
			Namespace: "default",
		},
	}

	id1 := generateUsageID(cr)
	id2 := generateUsageID(cr)

	if id1 != id2 {
		t.Error("generateUsageID should be consistent for same secret")
	}
	if id1 == "" {
		t.Error("generateUsageID should not return empty string")
	}
}

func TestGetUsageTypeVolume(t *testing.T) {
	usageType := getUsageType("volume", "test-vol")
	if usageType != "volume" {
		t.Errorf("Expected usage type 'volume', got: %s", usageType)
	}
}

func TestGetUsageTypeCeph(t *testing.T) {
	usageType := getUsageType("ceph", "ceph-user")
	if usageType != "ceph" {
		t.Errorf("Expected usage type 'ceph', got: %s", usageType)
	}
}

func TestGetUsageTypeISCSI(t *testing.T) {
	usageType := getUsageType("iscsi", "iscsi-user")
	if usageType != "iscsi" {
		t.Errorf("Expected usage type 'iscsi', got: %s", usageType)
	}
}

func TestGetUsageTypeTLS(t *testing.T) {
	usageType := getUsageType("tls", "server-cert")
	if usageType != "tls" {
		t.Errorf("Expected usage type 'tls', got: %s", usageType)
	}
}

func TestGetUsageElementNameVolume(t *testing.T) {
	elemName := getUsageElementName("volume")
	if elemName != "volume" {
		t.Errorf("Expected element name 'volume' for type 'volume', got: %s", elemName)
	}
}

func TestGetUsageElementNameCeph(t *testing.T) {
	elemName := getUsageElementName("ceph")
	if elemName != "name" {
		t.Errorf("Expected element name 'name' for type 'ceph', got: %s", elemName)
	}
}

func contains(s, substr string) bool {
	for i := 0; i < len(s)-len(substr)+1; i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
