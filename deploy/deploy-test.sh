#!/bin/bash

set -e

echo "🚀 Deploying provider-libvirt v0.2.1 with NodeDevice support..."

# Check if kubectl is available
if ! command -v kubectl &> /dev/null; then
    echo "❌ kubectl not found. Please install kubectl first."
    exit 1
fi

# Check if crossplane is installed
if ! kubectl get providers &> /dev/null; then
    echo "❌ Crossplane not found. Please install Crossplane first:"
    echo "   helm install crossplane --namespace crossplane-system --create-namespace crossplane-stable/crossplane"
    exit 1
fi

echo "📦 Installing provider-libvirt..."
kubectl apply -f provider-install.yaml

echo "⏳ Waiting for provider to be installed..."
kubectl wait --for=condition=Installed provider/provider-libvirt --timeout=300s

echo "✅ Provider installed successfully!"

echo "🔧 Checking provider status..."
kubectl describe provider provider-libvirt

echo ""
echo "📋 Next steps:"
echo "1. Update the libvirt-credentials secret with your actual connection URI"
echo "2. Deploy test NodeDevice resources:"
echo "   kubectl apply -f nodedevice-test.yaml"
echo ""
echo "🎯 Testing commands:"
echo "   # Check CRDs are installed"
echo "   kubectl get crd | grep nodedevice"
echo ""
echo "   # Monitor provider logs"
echo "   kubectl logs -n crossplane-system -l pkg.crossplane.io/provider=provider-libvirt -f"
echo ""
echo "   # Check NodeDevice resources"
echo "   kubectl get nodedevices"
echo ""
echo "🔐 Don't forget to:"
echo "   - Configure proper libvirt connection URI in the secret"
echo "   - Ensure VFIO is enabled on the host for GPU passthrough"
echo "   - Verify hardware device addresses match your system"