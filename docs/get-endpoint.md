# kubectl kaito get-endpoint

Get the inference endpoint URL for a deployed Kaito workspace.

## Synopsis

Get the inference endpoint URL for a deployed Kaito workspace. This command retrieves all available service endpoints that can be used to send inference requests to the deployed model. The endpoints support OpenAI-compatible APIs.

The command automatically discovers all accessible endpoints:

- **LoadBalancer**: Direct public access (if configured)
- **API Proxy**: Kubernetes API proxy (works anywhere kubectl works)  
- **Cluster-internal**: Direct cluster access (for pods only)

The URL format returns the best available endpoint (prefers external if available), while JSON format shows all discovered endpoints with detailed information.

## Usage

```bash
kaito get-endpoint [flags]
```

## Flags

| Flag                      | Type   | Default | Description                                  |
| ------------------------- | ------ | ------- | -------------------------------------------- |
| `--workspace-name string` | string |         | Name of the workspace (required)             |
| `-n, --namespace string`  | string |         | Kubernetes namespace                         |
| `--format string`         | string | json    | Output format: `json` or `text`              |

## Examples

### Basic Endpoint Retrieval

```bash
# Get endpoint URL for a workspace
kubectl kaito get-endpoint --workspace-name my-workspace
```

Output (from outside cluster):

```
https://your-api-server.com/api/v1/namespaces/default/services/my-workspace:80/proxy
```

### JSON Format Output - All Endpoints

```bash
# Get all available endpoints in JSON format
kubectl kaito get-endpoint --workspace-name my-workspace --format json
```

Output (showing all available endpoints):

```json
{
  "workspace": "my-workspace",
  "namespace": "default",
  "endpoints": [
    {
      "url": "https://your-api-server.com/api/v1/namespaces/default/services/my-workspace:80/proxy",
      "type": "APIProxy",
      "access": "cluster",
      "description": "Kubernetes API proxy (works anywhere kubectl works)"
    }
  ]
}
```

With LoadBalancer (if configured):

```json
{
  "workspace": "my-workspace", 
  "namespace": "default",
  "endpoints": [
    {
      "url": "http://203.0.113.42:80",
      "type": "LoadBalancer",
      "access": "external",
      "description": "Direct public access via LoadBalancer"
    },
    {
      "url": "https://your-api-server.com/api/v1/namespaces/default/services/my-workspace:80/proxy",
      "type": "APIProxy", 
      "access": "cluster",
      "description": "Kubernetes API proxy (works anywhere kubectl works)"
    }
  ]
}
```

## Endpoint Types

The command automatically discovers and returns all available endpoint types:

### API Proxy (cluster access)

- **Format**: `https://<api-server>/api/v1/namespaces/<namespace>/services/<workspace>:80/proxy`
- **Authentication**: Uses your kubectl credentials
- **Access**: Works anywhere kubectl works (local machine, CI/CD, etc.)
- **Security**: Authenticated via Kubernetes RBAC

### LoadBalancer (external access)

- **Format**: `http://<external-ip>:80`
- **Authentication**: None (direct access)
- **Access**: Public internet access
- **Security**: Unprotected (configure firewall rules as needed)
- **Availability**: Only if service type is LoadBalancer

### Cluster-Internal (pod access)

- **Format**: `http://<workspace>.<namespace>.svc.cluster.local:80`
- **Authentication**: None (direct access)
- **Access**: Only from within the Kubernetes cluster
- **Security**: Protected by cluster network policies
