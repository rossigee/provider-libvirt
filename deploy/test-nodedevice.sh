#!/bin/bash

set -e

echo "🧪 Testing NodeDevice functionality..."

# Function to check resource status
check_resource_status() {
    local resource=$1
    local name=$2
    echo "📊 Checking $resource/$name status..."
    kubectl get $resource $name -o yaml | grep -A 5 -B 5 "conditions\|status" || echo "No conditions found yet"
}

# Function to wait for resource to be ready
wait_for_ready() {
    local resource=$1
    local name=$2
    echo "⏳ Waiting for $resource/$name to be ready..."
    timeout 120s bash -c "until kubectl get $resource $name -o jsonpath='{.status.conditions[?(@.type==\"Ready\")].status}' | grep -q True; do sleep 5; done" || {
        echo "❌ Resource $resource/$name failed to become ready within 120 seconds"
        kubectl describe $resource $name
        return 1
    }
    echo "✅ $resource/$name is ready!"
}

# Test 1: GPU Passthrough
echo ""
echo "🎮 Test 1: GPU Passthrough"
echo "Applying GPU passthrough configuration..."
kubectl apply -f - <<EOF
apiVersion: nodedevice.libvirt.crossplane.io/v1alpha1
kind: NodeDevice
metadata:
  name: test-gpu-passthrough
spec:
  forProvider:
    name: test-gpu-device
    type: pci
    pciAddress:
      domain: "0x0000"
      bus: "0x01"
      slot: "0x00"
      function: "0x0"
    detached: true
    driver: vfio-pci
  providerConfigRef:
    name: default
  deletionPolicy: Delete
EOF

sleep 10
check_resource_status nodedevice test-gpu-passthrough

# Test 2: USB Device Management
echo ""
echo "🔌 Test 2: USB Device Management"
kubectl apply -f - <<EOF
apiVersion: nodedevice.libvirt.crossplane.io/v1alpha1
kind: NodeDevice
metadata:
  name: test-usb-device
spec:
  forProvider:
    name: test-usb-device
    type: usb
    usbDevice:
      vendorID: "1234"
      productID: "5678"
      bus: 1
      device: 2
    detached: false
  providerConfigRef:
    name: default
  deletionPolicy: Delete
EOF

sleep 10
check_resource_status nodedevice test-usb-device

# Test 3: Mediated Device (vGPU)
echo ""
echo "🖥️  Test 3: Mediated Device (vGPU)"
kubectl apply -f - <<EOF
apiVersion: nodedevice.libvirt.crossplane.io/v1alpha1
kind: NodeDevice
metadata:
  name: test-vgpu
spec:
  forProvider:
    type: mdev
    mediatedDevice:
      parent: pci_0000_02_00_0
      type: nvidia-63
    persistent: true
  providerConfigRef:
    name: default
  deletionPolicy: Delete
EOF

sleep 10
check_resource_status nodedevice test-vgpu

# Summary
echo ""
echo "📋 Test Summary:"
echo "==================================="
kubectl get nodedevices -o wide

echo ""
echo "🔍 Detailed Status Check:"
for device in test-gpu-passthrough test-usb-device test-vgpu; do
    echo ""
    echo "--- $device ---"
    kubectl get nodedevice $device -o jsonpath='{.status.conditions}' | jq . 2>/dev/null || echo "No status conditions"
done

echo ""
echo "🧹 Cleanup (optional):"
echo "kubectl delete nodedevice test-gpu-passthrough test-usb-device test-vgpu"
echo ""
echo "📊 Provider logs:"
echo "kubectl logs -n crossplane-system -l pkg.crossplane.io/provider=provider-libvirt --tail=50"