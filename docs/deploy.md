# kubectl kaito deploy

Deploy a Kaito workspace for AI model inference or fine-tuning.

## Synopsis

Deploy creates a new Kaito workspace resource for AI model deployment. This command supports both inference and fine-tuning scenarios:

- **Inference**: Deploy models for real-time inference with OpenAI-compatible APIs
- **Fine-tuning**: Fine-tune existing models with your own datasets using methods like QLoRA

The workspace will automatically provision the required GPU resources and deploy the specified model according to Kaito's preset configurations.

## Usage

```bash
kaito deploy [flags]
```

## Flags

### Required Flags

| Flag                      | Type   | Description                                |
| ------------------------- | ------ | ------------------------------------------ |
| `--workspace-name string` | string | Name of the workspace to create (required) |
| `--model string`          | string | Model name to deploy (required)            |
| `--instance-type string`  | string | GPU instance type (e.g., Standard_NC6s_v3) |

### Optional Flags

| Flag | Type | Default | Description |
| ---- | ---- | ------- | ----------- |

| `--count int`            | int    | 1       | Number of GPU nodes                                  |
| `--dry-run`              | bool   | false   | Show what would be created without actually creating |
| `--enable-load-balancer` | bool   | false   | Enable LoadBalancer service for external access      |
| `--node-selector stringToString` | map  | Node selector labels |

### Inference-Specific Flags

These flags can only be used when `--tuning` is **not** enabled (default inference mode):

| Flag                           | Type     | Description                     |
| ------------------------------ | -------- | ------------------------------- |
| `--model-access-secret string` | string   | Secret for private model access |
| `--adapters strings`           | []string | Model adapters to load          |
| `--inference-config string`    | string   | Custom inference configuration  |

### Fine-tuning Flags

These flags can only be used when `--tuning` is **enabled**:

| Flag                           | Type     | Default | Description                       |
| ------------------------------ | -------- | ------- | --------------------------------- |
| `--tuning`                     | bool     | false   | Enable fine-tuning mode           |
| `--tuning-method string`       | string   | qlora   | Fine-tuning method (qlora, lora)  |
| `--input-urls strings`         | []string |         | URLs to training data             |
| `--input-pvc string`           | string   |         | PVC containing training data      |
| `--output-image string`        | string   |         | Output image for fine-tuned model |
| `--output-pvc string`          | string   |         | PVC for output storage            |
| `--output-image-secret string` | string   |         | Secret for pushing output image   |
| `--tuning-config string`       | string   |         | Custom tuning configuration       |

> **Note**: You cannot mix inference and tuning flags. When `--tuning` is enabled, inference-specific flags (`--model-access-secret`, `--adapters`, `--inference-config`) cannot be used. When `--tuning` is not enabled, tuning-specific flags cannot be used.

## Examples

### Basic Inference Deployment

```bash
# Deploy Llama-3.1 8b for inference
kubectl kaito deploy --workspace-name llama-workspace \
--model llama-3.1-8b-instruct \
--model-access-secret hf-token
```

### Deployment with Specific Instance Type

```bash
# Deploy with specific instance type and count  
kubectl kaito deploy \
  --workspace-name phi-workspace \
  --model phi-3.5-mini-instruct \
  --instance-type Standard_NC6s_v3 \
  --count 2
```

### Fine-tuning Deployment

```bash
# Deploy for fine-tuning with QLoRA using URLs
kubectl kaito deploy \
  --workspace-name tune-phi \
  --model phi-3.5-mini-instruct \
  --tuning \
  --tuning-method qlora \
  --input-urls "https://example.com/data.parquet" \
  --output-image myregistry/phi-finetuned:latest

# Deploy for fine-tuning with PVC storage
kubectl kaito deploy \
  --workspace-name tune-llama \
  --model llama-3.1-8b-instruct \
  --tuning \
  --input-pvc training-data \
  --output-pvc model-output
```

### External Access Deployment

```bash
# Deploy with load balancer for external access
kubectl kaito deploy \
  --workspace-name public-llama \
  --model llama-3.1-8b-instruct \
  --enable-load-balancer
```

### Dry Run

```bash
# Preview what would be created
kubectl kaito deploy \
  --workspace-name test-workspace \
  --model phi-3.5-mini-instruct \
  --dry-run
```

### Node Selector Deployment

```bash
# Deploy on specific nodes
kubectl kaito deploy \
  --workspace-name selective-workspace \
  --model llama-2-7b \
  --node-selector gpu-type=A100,zone=us-west-2a
```

### LoadBalancer Deployment

```bash
# Deploy with LoadBalancer for external access
kubectl kaito deploy \
  --workspace-name public-llama \
  --model llama-3.1-8b-instruct \
  --enable-load-balancer
```

**Important Notes:**

- The `--enable-load-balancer` flag adds the `kaito.sh/enable-lb: "true"` annotation to the workspace
- This instructs the Kaito operator to create a LoadBalancer service for external access.
- Only works with inference workspaces (cannot be used with `--tuning`)
- May incur additional cloud provider costs for the LoadBalancer service

## Required Parameters by Mode

### Inference Mode (default)

- **Required**: `--workspace-name`, `--model`
- **Optional**: `--model-access-secret`, `--adapters`, `--inference-config`, `--instance-type`, `--count`, etc.

### Tuning Mode (`--tuning` enabled)

- **Required**: `--workspace-name`, `--model`, `--tuning`
- **Required (one of)**: `--input-urls` OR `--input-pvc`
- **Required (one of)**: `--output-image` OR `--output-pvc`
- **Optional**: `--tuning-method`, `--output-image-secret`, `--tuning-config`, `--instance-type`, `--count`, etc.
