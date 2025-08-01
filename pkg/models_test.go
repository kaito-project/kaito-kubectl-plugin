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
			name:        "Non-empty model name",
			modelName:   "some-model",
			expectError: false, // May still error if not in list, but should pass basic validation
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateModelName(tt.modelName)

			if tt.expectError && tt.modelName == "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), "cannot be empty")
			}
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
			assert.NotEmpty(t, model.Type)
			assert.NotEmpty(t, model.Runtime)
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
			name:     "Falcon model",
			input:    "falcon-7b",
			expected: "Falcon",
		},
		{
			name:     "Falcon instruct model",
			input:    "falcon-7b-instruct",
			expected: "Falcon",
		},
		{
			name:     "Llama model",
			input:    "llama-3.1-8b-instruct",
			expected: "Llama",
		},
		{
			name:     "Phi model",
			input:    "phi-3.5-mini-instruct",
			expected: "Phi",
		},
		{
			name:     "Phi simple model",
			input:    "phi-2",
			expected: "Phi",
		},
		{
			name:     "Mistral model",
			input:    "mistral-7b-instruct",
			expected: "Mistral",
		},
		{
			name:     "DeepSeek model",
			input:    "deepseek-r1-distill-llama-8b",
			expected: "DeepSeek",
		},
		{
			name:     "DeepSeek Qwen model",
			input:    "deepseek-r1-distill-qwen-14b",
			expected: "DeepSeek",
		},
		{
			name:     "Qwen2.5 coder model",
			input:    "qwen2.5-coder-7b-instruct",
			expected: "Qwen2.5",
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
