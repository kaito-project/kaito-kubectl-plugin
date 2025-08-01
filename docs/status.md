# kubectl kaito status

Check the status of one or more Kaito workspaces.

## Synopsis

Check the status of Kaito workspaces, displaying the current state of workspace resources, including readiness conditions, resource allocation, and deployment status.

## Usage

```bash
kaito status [flags]
```

## Flags

| Flag                      | Type   | Default | Description                            |
| ------------------------- | ------ | ------- | -------------------------------------- |
| `--workspace-name string` | string |         | Name of the workspace to check         |
| `-n, --namespace string`  | string |         | Kubernetes namespace                   |
| `-w, --watch`             | bool   | false   | Watch for changes in real-time         |

## Examples

### Check Specific Workspace

```bash
# Check status of a specific workspace
kubectl kaito status --workspace-name my-workspace
```

## Troubleshooting

### Common Status Issues

1. **RESOURCEREADY: False**
   - Check cluster has available GPU nodes
   - Verify instance type is available
   - Check node selectors and taints

2. **INFERENCEREADY: False**
   - Model might still be downloading
   - Check pod logs for model loading issues
   - Verify model configuration

3. **WORKSPACEREADY: False**
   - One or more conditions not met
