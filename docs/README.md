# Kaito CLI Documentation

The Kaito CLI (`kubectl-kaito`) is a command-line tool for managing AI/ML model inference and fine-tuning workloads using the Kubernetes AI Toolchain Operator (Kaito).

This plugin simplifies the deployment, management, and monitoring of AI models in Kubernetes clusters through Kaito workspaces.

## Quick Start

```bash
# Deploy a model for inference
kubectl kaito deploy --workspace-name my-llama --model llama-2-7b --instance-type Standard_NC6s_v3

# Check workspace status
kubectl kaito status --workspace-name my-llama

# Get model inference endpoint
kubectl kaito get-endpoint --workspace-name my-llama

# Interactive chat with deployed model
kubectl kaito chat --workspace-name my-llama

# List supported models
kubectl kaito models list
```

## Command Reference

- [**deploy**](./deploy.md) - Deploy a Kaito workspace for model inference or fine-tuning
- [**status**](./status.md) - Check status of Kaito workspaces
- [**get-endpoint**](./get-endpoint.md) - Get inference endpoints for a Kaito workspace
- [**chat**](./chat.md) - Interactive chat with deployed AI models
- [**models**](./models.md) - Manage and list supported AI models

## Global Flags

All commands support these global flags:

| Flag                     | Description                                          |
| ------------------------ | ---------------------------------------------------- |
| `--kubeconfig string`    | Path to the kubeconfig file to use for CLI requests  |
| `--context string`       | The name of the kubeconfig context to use            |
| `-n, --namespace string` | If present, the namespace scope for this CLI request |

## Installation

### Via Krew (Coming soon)

```bash
kubectl krew install kaito
```

### Manual Installation

1. Download the latest binary from the [releases page](https://github.com/kaito-project/kaito-kubectl-plugin/releases)
2. Place it in your `$PATH` as `kubectl-kaito`
3. Make it executable: `chmod +x kubectl-kaito`

## Examples

### Basic Inference Deployment

```bash
# Deploy Llama-2 7B model
kubectl kaito deploy --workspace-name llama-workspace --model llama-2-7b

# Wait for deployment to be ready
kubectl kaito status --workspace-name llama-workspace --watch

# Get the endpoint URL
kubectl kaito get-endpoint --workspace-name llama-workspace

# Start chatting with the model
kubectl kaito chat --workspace-name llama-workspace
```

### Fine-tuning Workflow

```bash
# Deploy a model for fine-tuning with QLoRA
kubectl kaito deploy \
  --workspace-name tune-phi \
  --model phi-3.5-mini-instruct \
  --tuning \
  --tuning-method qlora \
  --input-urls "https://example.com/training-data.parquet" \
  --output-image myregistry/phi-finetuned:latest

# Monitor the fine-tuning process
kubectl kaito status --workspace-name tune-phi --watch --show-conditions
```

### Multi-GPU Deployment

```bash
# Deploy with multiple GPU nodes for larger models
kubectl kaito deploy \
  --workspace-name large-llama \
  --model llama-2-70b \
  --instance-type Standard_NC24ads_A100_v4 \
  --count 4
```

## Prerequisites

- Kubernetes cluster with GPU nodes
- Kaito operator installed in the cluster
- kubectl configured to access the cluster
- Appropriate permissions to create and manage Kaito resources
