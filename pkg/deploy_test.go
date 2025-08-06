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
			name: "Tuning mode with model image",
			options: DeployOptions{
				WorkspaceName: "test-workspace",
				Model:         "phi-3.5-mini-instruct",
				Tuning:        true,
				TuningMethod:  "qlora",
				ModelImage:    "myregistry/base:latest",
				InputURLs:     []string{"https://example.com/data.parquet"},
				OutputImage:   "myregistry/model:latest",
			},
			expectError: false,
		},
		{
			name: "Inference mode with model image - should fail",
			options: DeployOptions{
				WorkspaceName: "test-workspace",
				Model:         "phi-3.5-mini-instruct",
				ModelImage:    "myregistry/base:latest", // This should cause validation to fail
			},
			expectError: true,
		},
		{
			name: "Tuning mode with empty model image",
			options: DeployOptions{
				WorkspaceName: "test-workspace",
				Model:         "phi-3.5-mini-instruct",
				Tuning:        true,
				TuningMethod:  "qlora",
				ModelImage:    "", // Empty model image is valid
				InputURLs:     []string{"https://example.com/data.parquet"},
				OutputImage:   "myregistry/model:latest",
			},
			expectError: false,
		},
		{
			name: "Tuning mode with all options",
			options: DeployOptions{
				WorkspaceName:     "test-workspace",
				Model:             "phi-3.5-mini-instruct",
				Tuning:            true,
				TuningMethod:      "qlora",
				ModelImage:        "myregistry/base:latest",
				InputURLs:         []string{"https://example.com/data.parquet"},
				OutputImage:       "myregistry/model:latest",
				OutputImageSecret: "my-secret",
				TuningConfig:      "my-config",
			},
			expectError: false,
		},
		{
			name: "Tuning mode with PVC and model image",
			options: DeployOptions{
				WorkspaceName: "test-workspace",
				Model:         "phi-3.5-mini-instruct",
				Tuning:        true,
				TuningMethod:  "qlora",
				ModelImage:    "myregistry/base:latest",
				InputPVC:      "input-pvc",
				OutputPVC:     "output-pvc",
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

func TestBuildWorkspaceWithModelImage(t *testing.T) {
	tests := []struct {
		name        string
		options     *DeployOptions
		expectImage bool
		imageName   string
	}{
		{
			name: "No model image",
			options: &DeployOptions{
				WorkspaceName: "test-workspace",
				Model:         "phi-3.5-mini-instruct",
				Tuning:        true,
			},
			expectImage: false,
		},
		{
			name: "With model image",
			options: &DeployOptions{
				WorkspaceName: "test-workspace",
				Model:         "phi-3.5-mini-instruct",
				Tuning:        true,
				ModelImage:    "myregistry/base:latest",
			},
			expectImage: true,
			imageName:   "myregistry/base:latest",
		},
		{
			name: "With model image and tuning config",
			options: &DeployOptions{
				WorkspaceName: "test-workspace",
				Model:         "phi-3.5-mini-instruct",
				Tuning:        true,
				ModelImage:    "myregistry/base:latest",
				TuningConfig:  "my-config",
			},
			expectImage: true,
			imageName:   "myregistry/base:latest",
		},
		{
			name: "With model image and all tuning options",
			options: &DeployOptions{
				WorkspaceName:     "test-workspace",
				Model:             "phi-3.5-mini-instruct",
				Tuning:            true,
				ModelImage:        "myregistry/base:latest",
				TuningConfig:      "my-config",
				InputURLs:         []string{"https://example.com/data.parquet"},
				OutputImage:       "myregistry/output:latest",
				OutputImageSecret: "my-secret",
			},
			expectImage: true,
			imageName:   "myregistry/base:latest",
		},
		{
			name: "With model image and PVC options",
			options: &DeployOptions{
				WorkspaceName: "test-workspace",
				Model:         "phi-3.5-mini-instruct",
				Tuning:        true,
				ModelImage:    "myregistry/base:latest",
				InputPVC:      "input-pvc",
				OutputPVC:     "output-pvc",
			},
			expectImage: true,
			imageName:   "myregistry/base:latest",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			workspace := tt.options.buildWorkspace()
			assert.NotNil(t, workspace)

			// Check if workspace has the correct structure
			assert.Equal(t, "kaito.sh/v1beta1", workspace.Object["apiVersion"])
			assert.Equal(t, "Workspace", workspace.Object["kind"])

			// Check tuning config (at root level, not under spec)
			tuning, ok := workspace.Object["tuning"].(map[string]interface{})
			assert.True(t, ok, "Expected tuning to be a map")

			preset, ok := tuning["preset"].(map[string]interface{})
			assert.True(t, ok, "Expected preset to be a map")

			// Check model image
			if tt.expectImage {
				presetOptions, ok := preset["presetOptions"].(map[string]interface{})
				assert.True(t, ok, "Expected presetOptions to be a map")
				image, exists := presetOptions["image"]
				assert.True(t, exists, "Expected image to be present")
				assert.Equal(t, tt.imageName, image)
			} else {
				_, exists := preset["presetOptions"]
				assert.False(t, exists, "Expected presetOptions to NOT be present")
			}

			// Check other tuning configuration
			if tt.options.TuningConfig != "" {
				config, exists := tuning["config"]
				assert.True(t, exists, "Expected tuning.config to be present")
				assert.Equal(t, tt.options.TuningConfig, config)
			}

			if len(tt.options.InputURLs) > 0 {
				input, ok := tuning["input"].(map[string]interface{})
				assert.True(t, ok, "Expected input to be a map")
				urls, exists := input["urls"]
				assert.True(t, exists, "Expected input.urls to be present")
				// URLs are converted to []interface{} for YAML marshaling
				urlsSlice, ok := urls.([]interface{})
				assert.True(t, ok, "Expected urls to be []interface{}")
				assert.Equal(t, len(tt.options.InputURLs), len(urlsSlice))
				for i, url := range tt.options.InputURLs {
					assert.Equal(t, url, urlsSlice[i])
				}
			}

			if tt.options.InputPVC != "" {
				input, ok := tuning["input"].(map[string]interface{})
				assert.True(t, ok, "Expected input to be a map")
				pvc, exists := input["pvc"]
				assert.True(t, exists, "Expected input.pvc to be present")
				assert.Equal(t, tt.options.InputPVC, pvc)
			}

			if tt.options.OutputImage != "" {
				output, ok := tuning["output"].(map[string]interface{})
				assert.True(t, ok, "Expected output to be a map")
				image, exists := output["image"]
				assert.True(t, exists, "Expected output.image to be present")
				assert.Equal(t, tt.options.OutputImage, image)
			}

			if tt.options.OutputPVC != "" {
				output, ok := tuning["output"].(map[string]interface{})
				assert.True(t, ok, "Expected output to be a map")
				pvc, exists := output["pvc"]
				assert.True(t, exists, "Expected output.pvc to be present")
				assert.Equal(t, tt.options.OutputPVC, pvc)
			}

			if tt.options.OutputImageSecret != "" {
				output, ok := tuning["output"].(map[string]interface{})
				assert.True(t, ok, "Expected output to be a map")
				secret, exists := output["imageSecret"]
				assert.True(t, exists, "Expected output.imageSecret to be present")
				assert.Equal(t, tt.options.OutputImageSecret, secret)
			}
		})
	}
}

func TestBuildWorkspaceWithEmptyInference(t *testing.T) {
	options := &DeployOptions{
		WorkspaceName: "test-workspace",
		Model:         "phi-3.5-mini-instruct",
		Namespace:     "default",
	}

	workspace := options.buildWorkspace()
	assert.NotNil(t, workspace)

	// Check if workspace has the correct structure
	assert.Equal(t, "kaito.sh/v1beta1", workspace.Object["apiVersion"])
	assert.Equal(t, "Workspace", workspace.Object["kind"])

	// Check inference config (at root level, not under spec)
	inference, ok := workspace.Object["inference"].(map[string]interface{})
	assert.True(t, ok, "Expected inference to be a map")
	assert.NotNil(t, inference, "Expected inference to be initialized")
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
			configName:   "my-config", // Uses the provided ConfigMap name directly
		},
		{
			name: "Inference config from YAML file",
			options: &DeployOptions{
				WorkspaceName:   "test-workspace",
				Model:           "phi-3.5-mini-instruct",
				Namespace:       "default",
				InferenceConfig: "/tmp/test_inference_config.yaml", // Must be an actual file path
			},
			expectConfig: true,
			configName:   "test-workspace-inference-config", // Creates a new ConfigMap with this name
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temporary file if needed
			if tt.name == "Inference config from YAML file" {
				// Create a temporary YAML file
				err := os.WriteFile("/tmp/test_inference_config.yaml", []byte("test: config"), 0644)
				assert.NoError(t, err)
				defer os.Remove("/tmp/test_inference_config.yaml")
			}

			// Initialize the workspace
			workspace := tt.options.buildWorkspace()
			assert.NotNil(t, workspace)

			// Check if workspace has the correct structure
			assert.Equal(t, "kaito.sh/v1beta1", workspace.Object["apiVersion"])
			assert.Equal(t, "Workspace", workspace.Object["kind"])

			// Check inference config (at root level, not under spec)
			inference, ok := workspace.Object["inference"].(map[string]interface{})
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

func TestBuildWorkspaceStructure(t *testing.T) {
	tests := []struct {
		name    string
		options *DeployOptions
	}{
		{
			name: "Inference workspace structure",
			options: &DeployOptions{
				WorkspaceName:     "test-workspace",
				Namespace:         "default",
				Model:             "llama-3.1-8b-instruct",
				InstanceType:      "Standard_NC64as_T4_v3",
				Count:             2,
				ModelAccessSecret: "hf-token",
				InferenceConfig:   "my-config",
			},
		},
		{
			name: "Tuning workspace structure",
			options: &DeployOptions{
				WorkspaceName:     "test-workspace",
				Namespace:         "default",
				Model:             "phi-3.5-mini-instruct",
				InstanceType:      "Standard_NC6s_v3",
				Tuning:            true,
				TuningMethod:      "qlora",
				InputURLs:         []string{"https://example.com/data.parquet"},
				OutputImage:       "myregistry/model:latest",
				OutputImageSecret: "my-secret",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			workspace := tt.options.buildWorkspace()
			assert.NotNil(t, workspace)

			// Check basic structure
			assert.Equal(t, "kaito.sh/v1beta1", workspace.Object["apiVersion"])
			assert.Equal(t, "Workspace", workspace.Object["kind"])

			// Check metadata
			metadata, ok := workspace.Object["metadata"].(map[string]interface{})
			assert.True(t, ok, "Expected metadata to be a map")
			assert.Equal(t, tt.options.WorkspaceName, metadata["name"])
			assert.Equal(t, tt.options.Namespace, metadata["namespace"])

			// Check resource field is at root level
			resource, ok := workspace.Object["resource"].(map[string]interface{})
			assert.True(t, ok, "Expected resource to be a map at root level")
			assert.Equal(t, tt.options.InstanceType, resource["instanceType"])

			if tt.options.Count > 0 {
				assert.Equal(t, int64(tt.options.Count), resource["count"])
			}

			labelSelector, ok := resource["labelSelector"].(map[string]interface{})
			assert.True(t, ok, "Expected labelSelector to be a map")
			matchLabels, ok := labelSelector["matchLabels"].(map[string]interface{})
			assert.True(t, ok, "Expected matchLabels to be a map")
			assert.NotNil(t, matchLabels)

			// Check inference or tuning is at root level
			if tt.options.Tuning {
				tuning, ok := workspace.Object["tuning"].(map[string]interface{})
				assert.True(t, ok, "Expected tuning to be a map at root level")
				assert.NotNil(t, tuning)

				// Verify inference is not present
				_, hasInference := workspace.Object["inference"]
				assert.False(t, hasInference, "Expected inference to NOT be present in tuning mode")
			} else {
				inference, ok := workspace.Object["inference"].(map[string]interface{})
				assert.True(t, ok, "Expected inference to be a map at root level")
				assert.NotNil(t, inference)

				// Verify tuning is not present
				_, hasTuning := workspace.Object["tuning"]
				assert.False(t, hasTuning, "Expected tuning to NOT be present in inference mode")
			}

			// Verify spec field is NOT present (fields should be at root level)
			_, hasSpec := workspace.Object["spec"]
			assert.False(t, hasSpec, "Expected spec field to NOT be present - fields should be at root level")
		})
	}
}
