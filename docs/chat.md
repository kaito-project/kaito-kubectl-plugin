# kubectl kaito chat

Start an interactive chat session with a deployed Kaito workspace model.

## Synopsis

Start an interactive chat session with a deployed Kaito workspace model. This command provides a chat interface to interact with deployed models using OpenAI-compatible APIs in interactive mode.

The command automatically handles endpoint detection and authentication:
- **LoadBalancer services**: Uses direct external access (if available)
- **ClusterIP services**: Uses Kubernetes API proxy (works anywhere kubectl works)  
- **Inside cluster**: Connects directly to cluster-internal service
- **No manual setup required**: No port-forwarding or additional configuration needed

## Usage

```bash
kubectl kaito chat [flags]
```

## Flags

| Flag                      | Type   | Default | Description                                   |
| ------------------------- | ------ | ------- | --------------------------------------------- |
| `--workspace-name string` | string |         | Name of the workspace (required)              |
| `-n, --namespace string`  | string |         | Kubernetes namespace                          |
| `--temperature float`     | float  | 0.7     | Temperature for response generation (0.0-2.0) |
| `--max-tokens int`        | int    | 1024    | Maximum tokens in response                    |
| `--top-p float`           | float  | 0.9     | Top-p (nucleus sampling) parameter (0.0-1.0)  |

## Examples

### Interactive Chat

```bash
# Start interactive chat session
kubectl kaito chat --workspace-name phi-3.5-mini-instruct
```

This opens an interactive session:
```
Connected to workspace: test-cli-gpu (model: phi-3.5-mini-instruct)
Type /help for commands or /quit to exit.

> Hello! How are you?
I'm Phi, an AI language model. I don't have feelings, but I'm here and ready to assist you. How can I help you today?

> What is machine learning?
Machine learning is a subset of artificial intelligence (AI) that involves the use of algorithms and statistical models to enable computers to improve at tasks with experience. In essence, machine learning systems are designed to learn from and make decisions or predictions based on data.

Here are some key points about machine learning:

1. Data-driven: Machine learning algorithms learn patterns and insights from large volumes of data. This data can be structured (like databases) or unstructured (like images, text, and sounds).
....

>/quit
```

### Configure Inference Parameters

```bash
# Configure inference parameters
kubectl kaito chat \
  --workspace-name my-llama \
  --temperature 0.5 \
  --max-tokens 512
```

## Interactive Commands

When in interactive mode, you can use these commands:

| Command          | Description                    |
| ---------------- | ------------------------------ |
| `quit` or `exit` | Exit the chat session          |
| `clear`         | Clear the conversation history |
| `help`          | Show available commands        |
| `status`       | Show current configuration     |

## Parameters

### Temperature (0.0 - 2.0)

Controls randomness in responses:

- **0.0**: Deterministic, always picks most likely response
- **0.7**: Balanced creativity and coherence (default)
- **1.0**: More creative and varied responses
- **2.0**: Highly random, may be incoherent

### Max Tokens

Maximum number of tokens in the response:

- **Default**: 1024 tokens
- **Range**: 1 - model's maximum context length
- **Note**: Includes both input and output tokens

### Top-p (0.0 - 1.0)

Nucleus sampling parameter:

- **0.1**: Very focused, uses only top 10% probable tokens
- **0.9**: Balanced selection (default)
- **1.0**: Consider all possible tokens
