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

func TestDeployCmd(t *testing.T) {
	configFlags := genericclioptions.NewConfigFlags(true)
	cmd := NewDeployCmd(configFlags)

	t.Run("Command structure", func(t *testing.T) {
		assert.Equal(t, "deploy", cmd.Use)
		assert.Contains(t, cmd.Short, "Deploy")
		assert.NotEmpty(t, cmd.Long)
		assert.NotEmpty(t, cmd.Example)
		assert.NotNil(t, cmd.RunE)
	})

	t.Run("Required flags present", func(t *testing.T) {
		flags := cmd.Flags()

		workspaceFlag := flags.Lookup("workspace-name")
		assert.NotNil(t, workspaceFlag)

		modelFlag := flags.Lookup("model")
		assert.NotNil(t, modelFlag)
	})
}

func TestDeployOptionsValidation(t *testing.T) {
	tests := []struct {
		name        string
		options     DeployOptions
		expectError bool
	}{
		{
			name: "Valid options",
			options: DeployOptions{
				WorkspaceName: "test-workspace",
				Model:         "phi-3.5-mini-instruct",
			},
			expectError: false,
		},
		{
			name: "Missing workspace name",
			options: DeployOptions{
				Model: "phi-3.5-mini-instruct",
			},
			expectError: true,
		},
		{
			name: "Missing model",
			options: DeployOptions{
				WorkspaceName: "test-workspace",
			},
			expectError: true,
		},
		{
			name: "Tuning mode with valid tuning flags",
			options: DeployOptions{
				WorkspaceName: "test-workspace",
				Model:         "phi-3.5-mini-instruct",
				Tuning:        true,
				TuningMethod:  "qlora",
				InputURLs:     []string{"https://example.com/data.parquet"},
				OutputImage:   "myregistry/model:latest",
			},
			expectError: false,
		},
		{
			name: "Inference mode with valid inference flags",
			options: DeployOptions{
				WorkspaceName:     "test-workspace",
				Model:             "phi-3.5-mini-instruct",
				ModelAccessSecret: "my-secret",
				Adapters:          []string{"adapter1", "adapter2"},
			},
			expectError: false,
		},
		{
			name: "Tuning mode with inference flags - should fail",
			options: DeployOptions{
				WorkspaceName:     "test-workspace",
				Model:             "phi-3.5-mini-instruct",
				Tuning:            true,
				TuningMethod:      "qlora",
				InputURLs:         []string{"https://example.com/data.parquet"},
				OutputImage:       "myregistry/model:latest",
				ModelAccessSecret: "my-secret", // This should cause validation to fail
			},
			expectError: true,
		},
		{
			name: "Inference mode with tuning flags - should fail",
			options: DeployOptions{
				WorkspaceName: "test-workspace",
				Model:         "phi-3.5-mini-instruct",
				InputURLs:     []string{"https://example.com/data.parquet"}, // This should cause validation to fail
			},
			expectError: true,
		},
		{
			name: "Tuning mode missing input data",
			options: DeployOptions{
				WorkspaceName: "test-workspace",
				Model:         "phi-3.5-mini-instruct",
				Tuning:        true,
				TuningMethod:  "qlora",
				OutputImage:   "myregistry/model:latest",
				// Missing InputURLs or InputPVC
			},
			expectError: true,
		},
		{
			name: "Tuning mode missing output",
			options: DeployOptions{
				WorkspaceName: "test-workspace",
				Model:         "phi-3.5-mini-instruct",
				Tuning:        true,
				TuningMethod:  "qlora",
				InputURLs:     []string{"https://example.com/data.parquet"},
				// Missing OutputImage or OutputPVC
			},
			expectError: true,
		},
		{
			name: "Tuning mode with PVC options",
			options: DeployOptions{
				WorkspaceName: "test-workspace",
				Model:         "phi-3.5-mini-instruct",
				Tuning:        true,
				TuningMethod:  "qlora",
				InputPVC:      "training-data",
				OutputPVC:     "model-output",
			},
			expectError: false,
		},
		{
			name: "Inference mode with LoadBalancer enabled",
			options: DeployOptions{
				WorkspaceName:      "test-workspace",
				Model:              "phi-3.5-mini-instruct",
				EnableLoadBalancer: true,
			},
			expectError: false,
		},
		{
			name: "Tuning mode with LoadBalancer - should fail",
			options: DeployOptions{
				WorkspaceName:      "test-workspace",
				Model:              "phi-3.5-mini-instruct",
				Tuning:             true,
				TuningMethod:       "qlora",
				InputURLs:          []string{"https://example.com/data.parquet"},
				OutputImage:        "myregistry/model:latest",
				EnableLoadBalancer: true,
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.options.Validate()

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestBuildWorkspaceWithLoadBalancer(t *testing.T) {
	tests := []struct {
		name               string
		enableLoadBalancer bool
		expectAnnotation   bool
	}{
		{
			name:               "LoadBalancer enabled",
			enableLoadBalancer: true,
			expectAnnotation:   true,
		},
		{
			name:               "LoadBalancer disabled",
			enableLoadBalancer: false,
			expectAnnotation:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			options := &DeployOptions{
				WorkspaceName:      "test-workspace",
				Model:              "phi-3.5-mini-instruct",
				Namespace:          "default",
				EnableLoadBalancer: tt.enableLoadBalancer,
				Count:              1,
			}

			workspace := options.buildWorkspace()

			// Check if workspace has the correct structure
			assert.Equal(t, "kaito.sh/v1beta1", workspace.Object["apiVersion"])
			assert.Equal(t, "Workspace", workspace.Object["kind"])

			// Check metadata
			metadata, ok := workspace.Object["metadata"].(map[string]interface{})
			assert.True(t, ok, "Expected metadata to be a map")

			// Check LoadBalancer annotation
			if tt.expectAnnotation {
				annotations, ok := metadata["annotations"].(map[string]interface{})
				assert.True(t, ok, "Expected annotations to be present when LoadBalancer is enabled")

				enableLB, exists := annotations["kaito.sh/enable-lb"]
				assert.True(t, exists, "Expected kaito.sh/enable-lb annotation to be present")
				assert.Equal(t, "true", enableLB, "Expected kaito.sh/enable-lb annotation to be 'true'")
			} else {
				// When LoadBalancer is disabled, annotations should either not exist or not contain the LB annotation
				if annotations, ok := metadata["annotations"].(map[string]interface{}); ok {
					_, exists := annotations["kaito.sh/enable-lb"]
					assert.False(t, exists, "Expected kaito.sh/enable-lb annotation to NOT be present when LoadBalancer is disabled")
				}
			}
		})
	}
}
