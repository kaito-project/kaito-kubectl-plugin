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

	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
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
				klog.Errorf("Validation failed: %v", err)
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
	cmd.Flags().StringVar(&o.InferenceConfig, "inference-config", "", "Custom inference configuration (either a ConfigMap name or path to a YAML file)")

	// Tuning specific flags
	cmd.Flags().BoolVar(&o.Tuning, "tuning", false, "Enable fine-tuning mode")
	cmd.Flags().StringVar(&o.TuningMethod, "tuning-method", "qlora", "Fine-tuning method (qlora, lora)")
	cmd.Flags().StringVar(&o.ModelImage, "model-image", "", "Custom image for the model preset")
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
		{"model-image", o.ModelImage, o.ModelImage == ""},
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

	// Create Kubernetes client
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		klog.Errorf("Failed to create Kubernetes client: %v", err)
		return fmt.Errorf("failed to create Kubernetes client: %w", err)
	}

	// Create ConfigMap if inference config is a file path
	if !o.Tuning && o.InferenceConfig != "" {
		// Check if it's a file path
		if _, statErr := os.Stat(o.InferenceConfig); statErr == nil {
			if createErr := createInferenceConfigMap(clientset, o.InferenceConfig, o.WorkspaceName, o.Namespace); createErr != nil {
				klog.Errorf("Failed to create inference ConfigMap: %v", createErr)
				return fmt.Errorf("failed to create inference ConfigMap: %w", createErr)
			}
		}
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

// buildWorkspace creates a new Workspace object with the specified configuration
func (o *DeployOptions) buildWorkspace() *unstructured.Unstructured {
	klog.V(4).Info("Building workspace configuration")

	// Create and initialize the workspace object
	workspace := o.initWorkspaceObject()

	// Set resource configuration
	o.setResourceConfig(workspace)

	// Configure inference or tuning
	if o.Tuning {
		o.setTuningConfig(workspace)
	} else {
		o.setInferenceConfig(workspace)
	}

	// Add LoadBalancer annotation if requested
	if o.EnableLoadBalancer {
		o.setLoadBalancerAnnotation(workspace)
	}

	return workspace
}

// initWorkspaceObject creates and initializes a new Workspace object with basic metadata
func (o *DeployOptions) initWorkspaceObject() *unstructured.Unstructured {
	workspace := &unstructured.Unstructured{
		Object: make(map[string]interface{}),
	}

	workspace.SetAPIVersion("kaito.sh/v1beta1")
	workspace.SetKind("Workspace")
	workspace.SetName(o.WorkspaceName)
	workspace.SetNamespace(o.Namespace)

	return workspace
}

// setResourceConfig sets the resource configuration at the root level
func (o *DeployOptions) setResourceConfig(workspace *unstructured.Unstructured) {
	resource := map[string]interface{}{
		"instanceType": o.InstanceType,
		"labelSelector": map[string]interface{}{
			"matchLabels": map[string]interface{}{
				"kaito.sh/workspace": o.WorkspaceName,
			},
		},
	}

	if o.Count > 0 {
		resource["count"] = int64(o.Count)
	}

	if len(o.LabelSelector) > 0 {
		resource["labelSelector"].(map[string]interface{})["matchLabels"] = o.LabelSelector
	}

	if err := unstructured.SetNestedField(workspace.Object, resource, "resource"); err != nil {
		klog.Errorf("Failed to set resource field: %v", err)
	}
}

// setTuningConfig sets the tuning configuration at the root level
func (o *DeployOptions) setTuningConfig(workspace *unstructured.Unstructured) {
	tuning := map[string]interface{}{}

	if o.TuningMethod != "" {
		tuning["method"] = o.TuningMethod
	}

	// Add model preset
	if o.Model != "" {
		preset := map[string]interface{}{
			"name": o.Model,
		}

		if o.ModelImage != "" {
			preset["presetOptions"] = map[string]interface{}{
				"image": o.ModelImage,
			}
		}

		tuning["preset"] = preset
	}

	// Add input configuration
	if len(o.InputURLs) > 0 {
		urls := make([]interface{}, len(o.InputURLs))
		for i, url := range o.InputURLs {
			urls[i] = url
		}
		tuning["input"] = map[string]interface{}{
			"urls": urls,
		}
	} else if o.InputPVC != "" {
		tuning["input"] = map[string]interface{}{
			"pvc": o.InputPVC,
		}
	}

	// Add output configuration
	if o.OutputImage != "" {
		tuning["output"] = map[string]interface{}{
			"image": o.OutputImage,
		}
	} else if o.OutputPVC != "" {
		tuning["output"] = map[string]interface{}{
			"pvc": o.OutputPVC,
		}
	}

	// Add output image secret if specified
	if o.OutputImageSecret != "" {
		if tuning["output"] == nil {
			tuning["output"] = map[string]interface{}{}
		}
		tuning["output"].(map[string]interface{})["imageSecret"] = o.OutputImageSecret
	}

	// Add tuning config if specified
	if o.TuningConfig != "" {
		tuning["config"] = o.TuningConfig
	}

	if err := unstructured.SetNestedField(workspace.Object, tuning, "tuning"); err != nil {
		klog.Errorf("Failed to set tuning field: %v", err)
	}
}

// setInferenceConfig sets the inference configuration at the root level
func (o *DeployOptions) setInferenceConfig(workspace *unstructured.Unstructured) {
	inference := map[string]interface{}{}

	// Add model preset
	if o.Model != "" {
		preset := map[string]interface{}{
			"name": o.Model,
		}

		if o.ModelAccessSecret != "" {
			preset["presetOptions"] = map[string]interface{}{
				"modelAccessSecret": o.ModelAccessSecret,
			}
		}

		inference["preset"] = preset
	}

	// Add adapters if specified
	if len(o.Adapters) > 0 {
		adapters := make([]interface{}, len(o.Adapters))
		for i, adapter := range o.Adapters {
			adapters[i] = adapter
		}
		inference["adapters"] = adapters
	}

	// Add inference config if specified
	if o.InferenceConfig != "" {
		// Check if it's a file path
		if _, statErr := os.Stat(o.InferenceConfig); statErr == nil {
			// Use the ConfigMap name that will be created
			configMapName := fmt.Sprintf("%s-inference-config", o.WorkspaceName)
			inference["config"] = configMapName
		} else {
			// Use the provided ConfigMap name directly
			inference["config"] = o.InferenceConfig
		}
	}

	if err := unstructured.SetNestedField(workspace.Object, inference, "inference"); err != nil {
		klog.Errorf("Failed to set inference field: %v", err)
	}
}

// setLoadBalancerAnnotation adds the LoadBalancer annotation to the workspace
func (o *DeployOptions) setLoadBalancerAnnotation(workspace *unstructured.Unstructured) {
	metadata := workspace.Object["metadata"].(map[string]interface{})
	if metadata["annotations"] == nil {
		metadata["annotations"] = map[string]interface{}{}
	}
	annotations := metadata["annotations"].(map[string]interface{})
	annotations["kaito.sh/enable-lb"] = "true"
	klog.V(4).Info("Added LoadBalancer annotation to workspace")
}

func createInferenceConfigMap(clientset kubernetes.Interface, configFile, workspaceName, namespace string) error {
	// Read the YAML file
	yamlData, err := os.ReadFile(configFile)
	if err != nil {
		return fmt.Errorf("failed to read inference config file: %w", err)
	}

	// Create a ConfigMap name from the workspace name
	configMapName := fmt.Sprintf("%s-inference-config", workspaceName)

	// Create the ConfigMap
	configMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      configMapName,
			Namespace: namespace,
		},
		Data: map[string]string{
			"inference_config.yaml": string(yamlData),
		},
	}

	// Create the ConfigMap
	_, err = clientset.CoreV1().ConfigMaps(namespace).Create(context.TODO(), configMap, metav1.CreateOptions{})
	if err != nil {
		if !errors.IsAlreadyExists(err) {
			return fmt.Errorf("failed to create ConfigMap: %w", err)
		}
		// If it already exists, update it
		_, err = clientset.CoreV1().ConfigMaps(namespace).Update(context.TODO(), configMap, metav1.UpdateOptions{})
		if err != nil {
			return fmt.Errorf("failed to update ConfigMap: %w", err)
		}
	}

	return nil
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
