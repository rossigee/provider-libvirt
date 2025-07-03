# Cross-Instance Validation

Provider-libvirt includes validation webhooks to prevent configuration errors when managing multiple libvirt instances. These webhooks ensure that resources referencing each other are on the same libvirt host.

## Overview

When managing multiple libvirt instances from a single Kubernetes cluster, it's critical to ensure that resources don't accidentally reference resources from different libvirt hosts. For example, a Domain on host1 cannot use a Volume that only exists on host2.

## Validation Rules

### Domain Resources

Domains are validated to ensure:
- They have a valid `providerConfigRef`
- Any referenced CloudInit disk uses the same `providerConfigRef`
- Future: Volume references will be validated once they become Kubernetes references instead of paths

Example of a validation error:
```
cloudinit disk my-cloudinit uses providerConfig libvirt-host1, but domain uses providerConfig libvirt-host2. Resources must use the same libvirt instance
```

### Volume Resources  

Volumes are validated to ensure:
- They have a valid `providerConfigRef`
- Any referenced Pool (by name) uses the same `providerConfigRef` if it exists as a Kubernetes resource

## Enabling Validation

Validation webhooks are disabled by default. To enable them:

1. Start the provider with webhook support:
   ```yaml
   spec:
     containers:
     - name: provider-libvirt
       args:
       - --enable-webhooks
       - --webhook-port=9443
   ```

2. Apply the webhook configuration:
   ```bash
   kubectl apply -f package/webhooks/webhook-config.yaml
   ```

3. Ensure the provider pod has the necessary certificates for webhook TLS (typically handled by cert-manager or similar)

## Best Practices

1. **Consistent Naming**: Use a naming convention that includes the target libvirt host
   - Good: `web-server-host1`, `database-host2`
   - Avoid: Generic names like `my-vm` that don't indicate the host

2. **Label Resources**: Add labels to identify which libvirt instance resources belong to
   ```yaml
   metadata:
     labels:
       libvirt.nourspeed.io/instance: host1
   ```

3. **Use Compositions**: Create compositions that enforce consistent `providerConfigRef` usage across related resources

## Limitations

- **Path-based References**: Volume paths (like `/var/lib/libvirt/images/disk1`) are not validated since they're strings, not Kubernetes references
- **Network References**: Network names are not validated as they're also string-based
- **Pre-existing Resources**: Resources that exist only on the libvirt side (not in Kubernetes) cannot be validated

## Troubleshooting

If you encounter validation errors:

1. Check that all related resources use the same `providerConfigRef`
2. Verify the referenced resources exist in Kubernetes
3. Use `kubectl describe` to see detailed validation error messages
4. Temporarily disable webhooks if you need to work around validation during migration

## Future Enhancements

- Validate network references once they become Kubernetes resources
- Add warnings for potential cross-instance issues with path-based references
- Support for resource migration between libvirt instances