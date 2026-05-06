/*
Copyright (c) 2024 Kaito Project

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/cli-runtime/pkg/genericclioptions"
)

func TestModelsCmd(t *testing.T) {
	configFlags := genericclioptions.NewConfigFlags(true)
	cmd := NewModelsCmd(configFlags)

	t.Run("Command structure", func(t *testing.T) {
		assert.Equal(t, "models", cmd.Use)
		assert.Contains(t, cmd.Short, "Manage")
		assert.NotEmpty(t, cmd.Long)
		assert.NotEmpty(t, cmd.Example)
	})

	t.Run("Subcommands present", func(t *testing.T) {
		subcommands := cmd.Commands()
		assert.Len(t, subcommands, 2)

		subcommandNames := make([]string, len(subcommands))
		for i, subcmd := range subcommands {
			subcommandNames[i] = subcmd.Name()
		}

		assert.Contains(t, subcommandNames, "list")
		assert.Contains(t, subcommandNames, "describe")
	})
}

func TestValidateModelName(t *testing.T) {
	tests := []struct {
		name        string
		modelName   string
		expectError bool
	}{
		{
			name:        "Empty model name",
			modelName:   "",
			expectError: true,
		},
		{
			name:        "Non-HuggingFace style model name",
			modelName:   "some-model",
			expectError: true, // All valid models now use org/model format
		},
		{
			name:        "HuggingFace model ID",
			modelName:   "Qwen/Qwen3-4B-Instruct-2507",
			expectError: false,
		},
		{
			name:        "HuggingFace model ID with nested org",
			modelName:   "microsoft/Phi-3.5-mini-instruct",
			expectError: false,
		},
		{
			name:        "Invalid HuggingFace model ID - missing model name",
			modelName:   "meta-llama/",
			expectError: true,
		},
		{
			name:        "Invalid HuggingFace model ID - missing org",
			modelName:   "/Llama-3.1-8B-Instruct",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateModelName(tt.modelName)

			if tt.expectError {
				assert.Error(t, err)
				if tt.modelName == "" {
					assert.Contains(t, err.Error(), "cannot be empty")
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestIsHuggingFaceModel(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{
			name:     "HuggingFace model ID",
			input:    "Qwen/Qwen3-4B-Instruct-2507",
			expected: true,
		},
		{
			name:     "Kaito preset model",
			input:    "llama-3.1-8b-instruct",
			expected: false,
		},
		{
			name:     "Empty string",
			input:    "",
			expected: false,
		},
		{
			name:     "HuggingFace with multiple slashes",
			input:    "org/sub/model",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsHuggingFaceModel(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGetSupportedModels(t *testing.T) {
	t.Run("Returns models", func(t *testing.T) {
		models := getSupportedModels()
		assert.NotEmpty(t, models)

		// Check that models have required fields
		for _, model := range models {
			assert.NotEmpty(t, model.Name)
		}
	})
}

func TestExtractModelFamily(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "HuggingFace model with org",
			input:    "microsoft/Phi-3.5-mini-instruct",
			expected: "Phi-3.5",
		},
		{
			name:     "HuggingFace meta-llama model",
			input:    "meta-llama/Llama-3.1-8B-Instruct",
			expected: "Llama-3.1",
		},
		{
			name:     "HuggingFace deepseek model",
			input:    "deepseek-ai/DeepSeek-R1-0528",
			expected: "DeepSeek",
		},
		{
			name:     "HuggingFace Qwen model",
			input:    "Qwen/Qwen2.5-Coder-7B-Instruct",
			expected: "Qwen2.5",
		},
		{
			name:     "Falcon model (legacy format)",
			input:    "falcon-7b",
			expected: "Falcon",
		},
		{
			name:     "Llama model (legacy format)",
			input:    "llama-3.1-8b-instruct",
			expected: "Llama-3.1",
		},
		{
			name:     "Phi model (legacy format)",
			input:    "phi-3.5-mini-instruct",
			expected: "Phi-3.5",
		},
		{
			name:     "DeepSeek model (legacy format)",
			input:    "deepseek-r1-distill-llama-8b",
			expected: "DeepSeek",
		},
		{
			name:     "Single word model",
			input:    "alpaca",
			expected: "Alpaca",
		},
		{
			name:     "Empty model name",
			input:    "",
			expected: "Unknown",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractModelFamily(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}
