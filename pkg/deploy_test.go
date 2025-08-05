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
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/kubernetes/fake"
)

func TestDeployCmd(t *testing.T) {
	t.Run("Inference config flag help text", func(t *testing.T) {
		configFlags := genericclioptions.NewConfigFlags(true)
		cmd := NewDeployCmd(configFlags)
		flag := cmd.Flags().Lookup("inference-config")
		assert.NotNil(t, flag)
		assert.Contains(t, flag.Usage, "ConfigMap name")
		assert.Contains(t, flag.Usage, "YAML file")
	})
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

func TestCreateInferenceConfigMap(t *testing.T) {
	tests := []struct {
		name        string
		options     *DeployOptions
		yamlContent string
		expectError bool
	}{
		{
			name: "Valid YAML file",
			options: &DeployOptions{
				WorkspaceName:   "test-workspace",
				Model:           "phi-3.5-mini-instruct",
				Namespace:       "default",
				InferenceConfig: "testdata/inference_config.yaml",
			},
			yamlContent: `vllm:
  cpu-offload-gb: 0
  gpu-memory-utilization: 0.95
  swap-space: 4
  max-model-len: 16384`,
			expectError: false,
		},
		{
			name: "Non-existent file",
			options: &DeployOptions{
				WorkspaceName:   "test-workspace",
				Model:           "phi-3.5-mini-instruct",
				Namespace:       "default",
				InferenceConfig: "testdata/nonexistent.yaml",
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var err error
			if tt.yamlContent != "" {
				// Create a temporary file with the YAML content
				tmpFile := fmt.Sprintf("/tmp/inference_config_%d.yaml", time.Now().UnixNano())
				err = os.WriteFile(tmpFile, []byte(tt.yamlContent), 0644)
				assert.NoError(t, err)
				defer os.Remove(tmpFile)

				// Update the options to use the temporary file
				tt.options.InferenceConfig = tmpFile
			}

			// Create a fake clientset
			clientset := fake.NewSimpleClientset()

			// Create the ConfigMap
			err = createInferenceConfigMap(clientset, tt.options.InferenceConfig, tt.options.WorkspaceName, tt.options.Namespace)

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)

				// Check if the ConfigMap was created correctly
				configMap, err := clientset.CoreV1().ConfigMaps(tt.options.Namespace).Get(context.TODO(), fmt.Sprintf("%s-inference-config", tt.options.WorkspaceName), metav1.GetOptions{})
				assert.NoError(t, err)
				assert.Equal(t, tt.yamlContent, configMap.Data["inference_config.yaml"])
			}
		})
	}
}

func TestBuildWorkspaceWithInferenceConfig(t *testing.T) {
	tests := []struct {
		name         string
		options      *DeployOptions
		expectConfig bool
		configName   string
	}{
		{
			name: "No inference config",
			options: &DeployOptions{
				WorkspaceName: "test-workspace",
				Model:         "phi-3.5-mini-instruct",
				Namespace:     "default",
			},
			expectConfig: false,
		},
		{
			name: "Inference config from ConfigMap name",
			options: &DeployOptions{
				WorkspaceName:   "test-workspace",
				Model:           "phi-3.5-mini-instruct",
				Namespace:       "default",
				InferenceConfig: "my-config",
			},
			expectConfig: true,
			configName:   "test-workspace-inference-config",
		},
		{
			name: "Inference config from YAML file",
			options: &DeployOptions{
				WorkspaceName:   "test-workspace",
				Model:           "phi-3.5-mini-instruct",
				Namespace:       "default",
				InferenceConfig: "testdata/inference_config.yaml",
			},
			expectConfig: true,
			configName:   "test-workspace-inference-config",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Initialize the workspace
			workspace := tt.options.buildWorkspace()
			assert.NotNil(t, workspace)

			// Initialize the workspace object if needed
			if workspace.Object == nil {
				workspace.Object = map[string]interface{}{
					"apiVersion": "kaito.sh/v1beta1",
					"kind":       "Workspace",
					"spec": map[string]interface{}{
						"inference": map[string]interface{}{},
					},
				}
			}

			// Check if workspace has the correct structure
			assert.Equal(t, "kaito.sh/v1beta1", workspace.Object["apiVersion"])
			assert.Equal(t, "Workspace", workspace.Object["kind"])

			// Check inference config
			spec, ok := workspace.Object["spec"].(map[string]interface{})
			assert.True(t, ok, "Expected spec to be a map")

			inference, ok := spec["inference"].(map[string]interface{})
			assert.True(t, ok, "Expected inference to be a map")

			if tt.expectConfig {
				config, exists := inference["config"]
				assert.True(t, exists, "Expected inference.config to be present")
				assert.Equal(t, tt.configName, config)
			} else {
				_, exists := inference["config"]
				assert.False(t, exists, "Expected inference.config to NOT be present")
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
