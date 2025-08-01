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

	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/dynamic"
	"k8s.io/klog/v2"
	"sigs.k8s.io/yaml"
)

// DeployOptions holds the options for the deploy command
type DeployOptions struct {
	configFlags        *genericclioptions.ConfigFlags
	Adapters           []string
	InputURLs          []string
	PreferredNodes     []string
	LabelSelector      map[string]string
	WorkspaceName      string
	Namespace          string
	Model              string
	InstanceType       string
	ModelAccessSecret  string
	InferenceConfig    string
	TuningMethod       string
	OutputImage        string
	OutputImageSecret  string
	TuningConfig       string
	InputPVC           string
	OutputPVC          string
	ModelAccessMode    string
	ModelImage         string
	Count              int
	DryRun             bool
	EnableLoadBalancer bool
	Tuning             bool
}

// NewDeployCmd creates the deploy command
func NewDeployCmd(configFlags *genericclioptions.ConfigFlags) *cobra.Command {
	o := &DeployOptions{
		configFlags: configFlags,
	}

	cmd := &cobra.Command{
		Use:   "deploy",
		Short: "Deploy a Kaito workspace for model inference or fine-tuning",
		Long: `Deploy creates a new Kaito workspace resource for AI model deployment.

This command supports both inference and fine-tuning scenarios:
- Inference: Deploy models for real-time inference with OpenAI-compatible APIs
- Tuning: Fine-tune existing models with your own datasets using methods like QLoRA

The workspace will automatically provision the required GPU resources and deploy
the specified model according to Kaito's preset configurations.`,
		Example: `  # Deploy Llama-2 7B for inference
  kubectl kaito deploy --workspace-name llama-workspace --model llama-2-7b

  # Deploy with specific instance type, count, and private model access
  kubectl kaito deploy --workspace-name phi-workspace --model phi-3.5-mini-instruct --instance-type Standard_NC6s_v3 --count 2 --model-access-secret my-secret

  # Deploy for fine-tuning with QLoRA (tuning mode)
  kubectl kaito deploy --workspace-name tune-phi --model phi-3.5-mini-instruct --tuning --tuning-method qlora --input-urls "https://example.com/data.parquet" --output-image myregistry/phi-finetuned:latest

  # Deploy for fine-tuning with PVC storage
  kubectl kaito deploy --workspace-name tune-llama --model llama-3.1-8b-instruct --tuning --input-pvc training-data --output-pvc model-output

  # Deploy with load balancer for external access (inference mode)
  kubectl kaito deploy --workspace-name public-llama --model llama-3.1-8b-instruct --enable-load-balancer`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := o.Validate(); err != nil {
				return err
			}
			return o.Run()
		},
	}

	// Required flags
	cmd.Flags().StringVar(&o.WorkspaceName, "workspace-name", "", "Name of the workspace to create (required)")
	cmd.Flags().StringVar(&o.Model, "model", "", "Model name to deploy (required)")

	// Resource configuration
	cmd.Flags().StringVar(&o.InstanceType, "instance-type", "", "GPU instance type (e.g., Standard_NC6s_v3)")
	cmd.Flags().IntVar(&o.Count, "count", 1, "Number of GPU nodes")
	cmd.Flags().StringToStringVar(&o.LabelSelector, "node-selector", nil, "Node selector labels")

	// Inference specific flags
	cmd.Flags().StringVar(&o.ModelAccessSecret, "model-access-secret", "", "Secret for private model access")
	cmd.Flags().StringSliceVar(&o.Adapters, "adapters", nil, "Model adapters to load")
	cmd.Flags().StringVar(&o.InferenceConfig, "inference-config", "", "Custom inference configuration")

	// Tuning specific flags
	cmd.Flags().BoolVar(&o.Tuning, "tuning", false, "Enable fine-tuning mode")
	cmd.Flags().StringVar(&o.TuningMethod, "tuning-method", "qlora", "Fine-tuning method (qlora, lora)")
	cmd.Flags().StringSliceVar(&o.InputURLs, "input-urls", nil, "URLs to training data")
	cmd.Flags().StringVar(&o.OutputImage, "output-image", "", "Output image for fine-tuned model")
	cmd.Flags().StringVar(&o.OutputImageSecret, "output-image-secret", "", "Secret for pushing output image")
	cmd.Flags().StringVar(&o.TuningConfig, "tuning-config", "", "Custom tuning configuration")
	cmd.Flags().StringVar(&o.InputPVC, "input-pvc", "", "PVC containing training data")
	cmd.Flags().StringVar(&o.OutputPVC, "output-pvc", "", "PVC for output storage")

	// Special options
	cmd.Flags().BoolVar(&o.DryRun, "dry-run", false, "Show what would be created without actually creating")
	cmd.Flags().BoolVar(&o.EnableLoadBalancer, "enable-load-balancer", false, "Create LoadBalancer service for external access")

	// Mark required flags
	if err := cmd.MarkFlagRequired("workspace-name"); err != nil {
		klog.Errorf("Failed to mark workspace-name flag as required: %v", err)
	}
	if err := cmd.MarkFlagRequired("model"); err != nil {
		klog.Errorf("Failed to mark model flag as required: %v", err)
	}

	return cmd
}

// Validate validates the deploy options
func (o *DeployOptions) Validate() error {
	klog.V(4).Info("Validating deploy options")

	if o.WorkspaceName == "" {
		return fmt.Errorf("workspace name is required")
	}
	if o.Model == "" {
		return fmt.Errorf("model name is required")
	}

	// Validate model name against official Kaito supported models
	if err := ValidateModelName(o.Model); err != nil {
		return err
	}

	// Check for conflicting inference/tuning parameters
	if err := o.validateModeFlags(); err != nil {
		return err
	}

	// Validate tuning specific requirements
	if o.Tuning {
		if len(o.InputURLs) == 0 && o.InputPVC == "" {
			return fmt.Errorf("tuning mode requires either --input-urls or --input-pvc")
		}
		if o.OutputImage == "" && o.OutputPVC == "" {
			return fmt.Errorf("tuning mode requires either --output-image or --output-pvc")
		}
	}

	klog.V(4).Info("Deploy options validation completed successfully")
	return nil
}

// validateModeFlags ensures users don't mix inference and tuning parameters
func (o *DeployOptions) validateModeFlags() error {
	// Define inference-specific flags
	inferenceFlags := []struct {
		name  string
		value interface{}
		empty bool
	}{
		{"model-access-secret", o.ModelAccessSecret, o.ModelAccessSecret == ""},
		{"adapters", o.Adapters, len(o.Adapters) == 0},
		{"inference-config", o.InferenceConfig, o.InferenceConfig == ""},
		{"enable-load-balancer", o.EnableLoadBalancer, !o.EnableLoadBalancer},
	}

	// Define tuning-specific flags (excluding tuning-method which has a default value)
	tuningFlags := []struct {
		name  string
		value interface{}
		empty bool
	}{
		{"input-urls", o.InputURLs, len(o.InputURLs) == 0},
		{"output-image", o.OutputImage, o.OutputImage == ""},
		{"output-image-secret", o.OutputImageSecret, o.OutputImageSecret == ""},
		{"tuning-config", o.TuningConfig, o.TuningConfig == ""},
		{"input-pvc", o.InputPVC, o.InputPVC == ""},
		{"output-pvc", o.OutputPVC, o.OutputPVC == ""},
	}

	// Check if tuning mode is explicitly enabled
	if o.Tuning {
		// In tuning mode, check if any inference-specific flags are set
		for _, flag := range inferenceFlags {
			if !flag.empty {
				return fmt.Errorf("cannot use inference flag --%s when --tuning is enabled", flag.name)
			}
		}
	} else {
		// In inference mode (default), check if any tuning-specific flags are set
		for _, flag := range tuningFlags {
			if !flag.empty {
				return fmt.Errorf("tuning flag --%s can only be used with --tuning enabled", flag.name)
			}
		}
	}

	return nil
}

// Run executes the deploy command
func (o *DeployOptions) Run() error {
	klog.V(2).Infof("Starting deploy command for workspace: %s", o.WorkspaceName)

	if err := o.Validate(); err != nil {
		klog.Errorf("Validation failed: %v", err)
		return fmt.Errorf("validation failed: %w", err)
	}

	// Get namespace from config flags if not set
	if o.Namespace == "" {
		if ns, _, err := o.configFlags.ToRawKubeConfigLoader().Namespace(); err == nil && ns != "" {
			o.Namespace = ns
		} else {
			klog.V(4).Info("No namespace specified, using 'default'")
			o.Namespace = "default"
		}
	}

	if o.DryRun {
		return o.showDryRun()
	}

	// Get REST config
	config, err := o.configFlags.ToRESTConfig()
	if err != nil {
		klog.Errorf("Failed to get REST config: %v", err)
		return fmt.Errorf("failed to get REST config: %w", err)
	}

	// Create dynamic client
	dynamicClient, err := dynamic.NewForConfig(config)
	if err != nil {
		klog.Errorf("Failed to create dynamic client: %v", err)
		return fmt.Errorf("failed to create dynamic client: %w", err)
	}

	// Create workspace
	workspace := o.buildWorkspace()

	klog.V(2).Infof("Creating workspace %s in namespace %s", o.WorkspaceName, o.Namespace)

	gvr := schema.GroupVersionResource{
		Group:    "kaito.sh",
		Version:  "v1beta1",
		Resource: "workspaces",
	}

	_, err = dynamicClient.Resource(gvr).Namespace(o.Namespace).Create(
		context.TODO(),
		workspace,
		metav1.CreateOptions{},
	)

	if err != nil {
		if errors.IsAlreadyExists(err) {
			fmt.Printf("✓ Workspace %s already exists\n", o.WorkspaceName)
			return nil
		}
		klog.Errorf("Failed to create workspace: %v", err)
		return fmt.Errorf("failed to create workspace: %w", err)
	}

	fmt.Printf("✓ Workspace %s created successfully\n", o.WorkspaceName)
	fmt.Printf("ℹ️  Use 'kubectl kaito status --workspace-name %s' to check status\n", o.WorkspaceName)
	return nil
}

func (o *DeployOptions) buildWorkspace() *unstructured.Unstructured {
	klog.V(4).Info("Building workspace configuration")

	// Create the base workspace object
	workspace := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "kaito.sh/v1beta1",
			"kind":       "Workspace",
			"metadata": map[string]interface{}{
				"name":      o.WorkspaceName,
				"namespace": o.Namespace,
			},
		},
	}

	// Add LoadBalancer annotation if requested
	if o.EnableLoadBalancer {
		metadata := workspace.Object["metadata"].(map[string]interface{})
		if metadata["annotations"] == nil {
			metadata["annotations"] = map[string]interface{}{}
		}
		annotations := metadata["annotations"].(map[string]interface{})
		annotations["kaito.sh/enable-lb"] = "true"
		klog.V(4).Info("Added LoadBalancer annotation to workspace")
	}

	// Add the spec fields at the top level (not inside a spec field)
	spec := o.createWorkspaceSpec()
	for key, value := range spec {
		workspace.Object[key] = value
	}

	return workspace
}

func (o *DeployOptions) createWorkspaceSpec() map[string]interface{} {
	klog.V(4).Info("Creating workspace specification")

	spec := map[string]interface{}{
		"resource": map[string]interface{}{
			"instanceType": o.InstanceType,
		},
	}

	// Add node count if specified
	if o.Count > 0 {
		spec["resource"].(map[string]interface{})["count"] = o.Count
		klog.V(4).Infof("Set node count to %d", o.Count)
	}

	// Add label selector - use provided one or create a default
	var labelSelector map[string]interface{}
	if len(o.LabelSelector) > 0 {
		labelSelector = map[string]interface{}{
			"matchLabels": o.LabelSelector,
		}
		klog.V(4).Infof("Added label selector: %v", o.LabelSelector)
	} else {
		// Default label selector using workspace name
		labelSelector = map[string]interface{}{
			"matchLabels": map[string]interface{}{
				"kaito.sh/workspace": o.WorkspaceName,
			},
		}
		klog.V(4).Infof("Added default label selector for workspace: %s", o.WorkspaceName)
	}
	spec["resource"].(map[string]interface{})["labelSelector"] = labelSelector

	// Configure inference or tuning
	if o.Tuning {
		klog.V(3).Info("Configuring tuning mode")
		// Tuning configuration
		tuning := map[string]interface{}{}

		if o.TuningMethod != "" {
			tuning["method"] = o.TuningMethod
		}

		if o.Model != "" {
			tuning["preset"] = map[string]interface{}{
				"name": o.Model,
			}
		}

		if len(o.InputURLs) > 0 {
			tuning["input"] = map[string]interface{}{
				"urls": o.InputURLs,
			}
		} else if o.InputPVC != "" {
			tuning["input"] = map[string]interface{}{
				"pvc": o.InputPVC,
			}
		}

		if o.OutputImage != "" {
			tuning["output"] = map[string]interface{}{
				"image": o.OutputImage,
			}
		} else if o.OutputPVC != "" {
			tuning["output"] = map[string]interface{}{
				"pvc": o.OutputPVC,
			}
		}

		if o.OutputImageSecret != "" {
			if tuning["output"] == nil {
				tuning["output"] = map[string]interface{}{}
			}
			tuning["output"].(map[string]interface{})["imageSecret"] = o.OutputImageSecret
		}

		if o.TuningConfig != "" {
			tuning["config"] = o.TuningConfig
		}

		spec["tuning"] = tuning
	} else {
		klog.V(3).Info("Configuring inference mode")
		// Inference configuration
		inference := map[string]interface{}{}

		if o.Model != "" {
			inference["preset"] = map[string]interface{}{
				"name": o.Model,
			}
		}

		// Add model access secret if specified
		if o.ModelAccessSecret != "" {
			inference["accessMode"] = "private"
			inference["secretName"] = o.ModelAccessSecret
			klog.V(4).Info("Added private model access configuration")
		}

		// Add adapters if specified
		if len(o.Adapters) > 0 {
			inference["adapters"] = o.Adapters
			klog.V(4).Infof("Added adapters: %v", o.Adapters)
		}

		// Add inference config if specified
		if o.InferenceConfig != "" {
			inference["config"] = o.InferenceConfig
			klog.V(4).Info("Added custom inference configuration")
		}

		spec["inference"] = inference
	}

	return spec
}

func (o *DeployOptions) showDryRun() error {
	klog.V(2).Info("Running in dry-run mode")

	fmt.Println("🔍 Dry-run mode: Showing what would be created")
	fmt.Println()
	fmt.Println("Workspace Configuration:")
	fmt.Println("========================")
	fmt.Printf("Name: %s\n", o.WorkspaceName)
	fmt.Printf("Namespace: %s\n", o.Namespace)
	fmt.Printf("Model: %s\n", o.Model)
	fmt.Printf("Count: %d\n", o.Count)

	if o.InstanceType != "" {
		fmt.Printf("Instance Type: %s\n", o.InstanceType)
	}

	if o.Tuning {
		fmt.Printf("Mode: Fine-tuning (%s)\n", o.TuningMethod)
		if len(o.InputURLs) > 0 {
			fmt.Printf("Input URLs: %v\n", o.InputURLs)
		}
		if o.InputPVC != "" {
			fmt.Printf("Input PVC: %s\n", o.InputPVC)
		}
		if o.OutputImage != "" {
			fmt.Printf("Output Image: %s\n", o.OutputImage)
		}
		if o.OutputPVC != "" {
			fmt.Printf("Output PVC: %s\n", o.OutputPVC)
		}
		if o.OutputImageSecret != "" {
			fmt.Printf("Output Image Secret: %s\n", o.OutputImageSecret)
		}
		if o.TuningConfig != "" {
			fmt.Printf("Tuning Config: %s\n", o.TuningConfig)
		}
	} else {
		fmt.Println("Mode: Inference")
		if len(o.Adapters) > 0 {
			fmt.Printf("Adapters: %v\n", o.Adapters)
		}
		if o.ModelAccessSecret != "" {
			fmt.Printf("Model Access Secret: %s\n", o.ModelAccessSecret)
		}
		if o.InferenceConfig != "" {
			fmt.Printf("Inference Config: %s\n", o.InferenceConfig)
		}
		if o.EnableLoadBalancer {
			fmt.Println("LoadBalancer: Enabled")
		}
	}

	if len(o.LabelSelector) > 0 {
		fmt.Printf("Label Selector: %v\n", o.LabelSelector)
	}

	fmt.Println()
	fmt.Println("✓ Workspace definition is valid")

	// Also show the actual workspace YAML that would be created
	workspace := o.buildWorkspace()

	// Convert to YAML for display
	yamlData, err := yaml.Marshal(workspace.Object)
	if err != nil {
		klog.Errorf("Failed to marshal workspace to YAML: %v", err)
	} else {
		fmt.Println()
		fmt.Println("Workspace YAML:")
		fmt.Println("===============")
		fmt.Printf("%s", string(yamlData))
	}

	fmt.Println("ℹ️  Run without --dry-run to create the workspace")

	return nil
}
