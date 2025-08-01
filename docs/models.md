# kubectl kaito models

Manage and list supported AI models available in Kaito.

## Synopsis

List and describe supported AI models available in Kaito. This command helps you discover which models are supported, their requirements, and configuration options for deployment. The model list is fetched from the official Kaito repository to ensure accuracy.

## Usage

```bash
kaito models [command]
```

## Available Commands

- [`list`](#list) - List supported AI models
- [`describe`](#describe) - Describe a specific AI model

---

## list

List all supported AI models available for deployment with Kaito.

### Usage

```bash
kaito models list [flags]
```

### Flags

| Flag               | Type     | Default | Description                                  |
| ------------------ | -------- | ------- | -------------------------------------------- |
| `--detailed`       | bool     | false   | Show detailed model information              |
| `--output`         | bool     | false   | Output in JSON format                        |

### Examples

#### Basic Model List

```bash
# List all models
kubectl kaito models list
```

Output:
```shell
NAME                          TYPE             FAMILY    RUNTIME  TAG
deepseek-r1-distill-llama-8b  text-generation  DeepSeek  tfs      0.2.0
deepseek-r1-distill-qwen-14b  text-generation  DeepSeek  tfs      0.2.0
falcon-40b                    text-generation  Falcon    tfs      0.2.0
falcon-40b-instruct           text-generation  Falcon    tfs      0.2.0
falcon-7b                     text-generation  Falcon    tfs      0.2.0
falcon-7b-instruct            text-generation  Falcon    tfs      0.2.0
llama-3.1-8b-instruct         text-generation  Llama     tfs      0.2.0
mistral-7b-instruct           text-generation  Mistral   tfs      0.2.0
phi-3.5-mini-instruct         text-generation  Phi       tfs      0.2.0

💡 Note: For deployment guidance and instanceType requirements,
   use 'kubectl kaito models describe <model>' or refer to Kaito workspace examples.
```

---

## describe

Describe a specific AI model in detail.

### Usage

```bash
kaito models describe [model-name]
```

### Examples

#### Describe Specific Model

```bash
# Describe a specific model
kubectl kaito models describe phi-3.5-mini-instruct
```

Output:
```shell
Model: phi-3.5-mini-instruct
================
Description: Official Kaito supported model: phi-3.5-mini-instruct
Type: text-generation
Runtime: tfs
Version: https://huggingface.co/microsoft/Phi-3.5-mini-instruct/commit/3145e03a9fd4cdd7cd953c34d9bbf7ad606122ca

Resource Requirements:
  💡 GPU requirements are not available in the official Kaito repository.
     For instanceType guidance, refer to:
     - Kaito workspace examples in the GitHub repository
     - Azure VM sizes documentation
     - Hugging Face model cards for model sizes
     - Community benchmarks

Usage Example:
  kubectl kaito deploy --workspace-name my-workspace --model phi-3.5-mini-instruct
```
