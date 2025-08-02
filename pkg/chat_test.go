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

func TestNewChatCmd(t *testing.T) {
	configFlags := genericclioptions.NewConfigFlags(true)
	cmd := NewChatCmd(configFlags)

	t.Run("Command structure", func(t *testing.T) {
		assert.Equal(t, "chat", cmd.Use)
		assert.Contains(t, cmd.Short, "Interactive chat")
		assert.NotEmpty(t, cmd.Long)
		assert.NotEmpty(t, cmd.Example)
		assert.NotNil(t, cmd.RunE)
	})

	t.Run("Required flags", func(t *testing.T) {
		flags := cmd.Flags()

		requiredFlags := []string{
			"workspace-name",
		}

		for _, flagName := range requiredFlags {
			flag := flags.Lookup(flagName)
			assert.NotNil(t, flag, "Required flag %s should be present", flagName)
		}
	})

	t.Run("Optional flags", func(t *testing.T) {
		flags := cmd.Flags()

		optionalFlags := []string{
			"temperature",
			"top-p",
			"max-tokens",
		}

		for _, flagName := range optionalFlags {
			flag := flags.Lookup(flagName)
			assert.NotNil(t, flag, "Optional flag %s should be present", flagName)
		}
	})

	t.Run("Flag aliases", func(t *testing.T) {
		flags := cmd.Flags()

		// Check short alias for namespace
		namespaceFlag := flags.ShorthandLookup("n")
		assert.NotNil(t, namespaceFlag)
		assert.Equal(t, "namespace", namespaceFlag.Name)
	})
}

func TestChatOptionsValidation(t *testing.T) {
	tests := []struct {
		name        string
		options     ChatOptions
		expectError bool
		errorMsg    string
	}{
		{
			name: "Valid options",
			options: ChatOptions{
				WorkspaceName: "test-workspace",
				Temperature:   0.7,
				TopP:          0.9,
				MaxTokens:     1024,
			},
			expectError: false,
		},
		{
			name: "Missing workspace name",
			options: ChatOptions{
				Temperature: 0.7,
				TopP:        0.9,
				MaxTokens:   1024,
			},
			expectError: true,
			errorMsg:    "workspace name is required",
		},
		{
			name: "Temperature too low",
			options: ChatOptions{
				WorkspaceName: "test-workspace",
				Temperature:   -0.1,
				TopP:          0.9,
				MaxTokens:     1024,
			},
			expectError: true,
			errorMsg:    "temperature must be between 0.0 and 2.0",
		},
		{
			name: "Temperature too high",
			options: ChatOptions{
				WorkspaceName: "test-workspace",
				Temperature:   2.1,
				TopP:          0.9,
				MaxTokens:     1024,
			},
			expectError: true,
			errorMsg:    "temperature must be between 0.0 and 2.0",
		},
		{
			name: "TopP too low",
			options: ChatOptions{
				WorkspaceName: "test-workspace",
				Temperature:   0.7,
				TopP:          -0.1,
				MaxTokens:     1024,
			},
			expectError: true,
			errorMsg:    "top-p must be between 0.0 and 1.0",
		},
		{
			name: "TopP too high",
			options: ChatOptions{
				WorkspaceName: "test-workspace",
				Temperature:   0.7,
				TopP:          1.1,
				MaxTokens:     1024,
			},
			expectError: true,
			errorMsg:    "top-p must be between 0.0 and 1.0",
		},
		{
			name: "MaxTokens too low",
			options: ChatOptions{
				WorkspaceName: "test-workspace",
				Temperature:   0.7,
				TopP:          0.9,
				MaxTokens:     0,
			},
			expectError: true,
			errorMsg:    "max-tokens must be greater than 0",
		},
		{
			name: "Valid edge values",
			options: ChatOptions{
				WorkspaceName: "test-workspace",
				Temperature:   0.0,
				TopP:          0.0,
				MaxTokens:     1,
			},
			expectError: false,
		},
		{
			name: "Valid max values",
			options: ChatOptions{
				WorkspaceName: "test-workspace",
				Temperature:   2.0,
				TopP:          1.0,
				MaxTokens:     4096,
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.options.validate()

			if tt.expectError {
				assert.Error(t, err)
				if tt.errorMsg != "" {
					assert.Contains(t, err.Error(), tt.errorMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestChatOptionsDefaults(t *testing.T) {
	t.Run("Default values", func(t *testing.T) {
		options := ChatOptions{}

		assert.Empty(t, options.WorkspaceName)
		assert.Empty(t, options.Namespace)

		assert.Equal(t, 0.0, options.Temperature) // Will be set to 0.7 in NewChatCmd
		assert.Equal(t, 0, options.MaxTokens)     // Will be set to 1024 in NewChatCmd
		assert.Equal(t, 0.0, options.TopP)        // Will be set to 0.9 in NewChatCmd
	})
}

func TestChatParameterValidation(t *testing.T) {
	t.Run("Temperature parameter validation", func(t *testing.T) {
		options := &ChatOptions{
			WorkspaceName: "test",
			MaxTokens:     1024, // Set valid max tokens
			TopP:          0.9,  // Set valid top-p
		}

		// Test valid temperature
		options.Temperature = 0.7
		assert.NoError(t, options.validate())

		// Test edge cases
		options.Temperature = 0.0
		assert.NoError(t, options.validate())

		options.Temperature = 2.0
		assert.NoError(t, options.validate())
	})

	t.Run("TopP parameter validation", func(t *testing.T) {
		options := &ChatOptions{
			WorkspaceName: "test",
			Temperature:   0.7,
			MaxTokens:     1024, // Set valid max tokens
		}

		// Test valid top-p
		options.TopP = 0.9
		assert.NoError(t, options.validate())

		// Test edge cases
		options.TopP = 0.0
		assert.NoError(t, options.validate())

		options.TopP = 1.0
		assert.NoError(t, options.validate())
	})

	t.Run("MaxTokens parameter validation", func(t *testing.T) {
		options := &ChatOptions{
			WorkspaceName: "test",
			Temperature:   0.7,
			TopP:          0.9,
		}

		// Test valid max tokens
		options.MaxTokens = 1024
		assert.NoError(t, options.validate())

		options.MaxTokens = 1
		assert.NoError(t, options.validate())
	})
}

func TestChatOptionsMethodsExist(t *testing.T) {
	configFlags := genericclioptions.NewConfigFlags(true)
	options := &ChatOptions{
		configFlags:   configFlags,
		WorkspaceName: "test-workspace",
		Temperature:   0.7,
		TopP:          0.9,
		MaxTokens:     1024,
	}

	t.Run("validate method", func(t *testing.T) {
		err := options.validate()
		assert.NoError(t, err)
	})

	t.Run("methods exist and are callable", func(t *testing.T) {
		// Test that we can call the parameter setting method
		assert.NotPanics(t, func() {
			options.setParameter("temperature", "0.8")
		})

		// Verify the parameter was set
		assert.Equal(t, 0.8, options.Temperature)
	})
}

func TestChatCommandIntegration(t *testing.T) {
	t.Run("Command with default options", func(t *testing.T) {
		configFlags := genericclioptions.NewConfigFlags(true)
		cmd := NewChatCmd(configFlags)

		// Test that the command is properly configured
		assert.Equal(t, "chat", cmd.Use)
		assert.NotEmpty(t, cmd.Short)
		assert.NotEmpty(t, cmd.Long)
		assert.NotEmpty(t, cmd.Example)

		// Test that default values are set correctly in the command
		flags := cmd.Flags()

		tempFlag := flags.Lookup("temperature")
		assert.NotNil(t, tempFlag)
		assert.Equal(t, "0.7", tempFlag.DefValue)

		maxTokensFlag := flags.Lookup("max-tokens")
		assert.NotNil(t, maxTokensFlag)
		assert.Equal(t, "1024", maxTokensFlag.DefValue)

		topPFlag := flags.Lookup("top-p")
		assert.NotNil(t, topPFlag)
		assert.Equal(t, "0.9", topPFlag.DefValue)
	})
}
