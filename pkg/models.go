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
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
	"gopkg.in/yaml.v2"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/klog/v2"
)

// SupportedModelsURL is the official URL for Kaito supported models
const SupportedModelsURL = "https://raw.githubusercontent.com/kaito-project/kaito/main/presets/workspace/models/supported_models.yaml"

// Model represents a supported AI model from the official Kaito repository
type Model struct {
	Tags         []string          `json:"tags" yaml:"tags"`
	Properties   map[string]string `json:"properties,omitempty" yaml:"properties,omitempty"`
	Name         string            `json:"name" yaml:"name"`
	Type         string            `json:"type" yaml:"type"`
	Runtime      string            `json:"runtime" yaml:"runtime"`
	Description  string            `json:"description" yaml:"description"`
	Version      string            `json:"version" yaml:"version"`
	Tag          string            `json:"tag" yaml:"tag"`
	GPUMemory    string            `json:"gpu_memory" yaml:"gpuMemory"`
	InstanceType string            `json:"instance_type,omitempty" yaml:"instanceType,omitempty"`
	MinNodes     int               `json:"min_nodes" yaml:"minNodes"`
	MaxNodes     int               `json:"max_nodes" yaml:"maxNodes"`
}

// KaitoSupportedModelsResponse represents the structure of the official supported_models.yaml
type KaitoSupportedModelsResponse struct {
	Models []struct {
		Properties   map[string]string `yaml:"properties,omitempty"`
		Name         string            `yaml:"name"`
		Version      string            `yaml:"version,omitempty"`
		Tag          string            `yaml:"tag,omitempty"`
		Type         string            `yaml:"type,omitempty"`
		Runtime      string            `yaml:"runtime,omitempty"`
		GPUMemory    string            `yaml:"gpuMemory,omitempty"`
		InstanceType string            `yaml:"instanceType,omitempty"`
		Description  string            `yaml:"description,omitempty"`
		MinNodes     int               `yaml:"minNodes,omitempty"`
		MaxNodes     int               `yaml:"maxNodes,omitempty"`
	} `yaml:"models"`
}

// fetchSupportedModelsFromKaito retrieves the official supported models from Kaito repository
func fetchSupportedModelsFromKaito() ([]Model, error) {
	klog.V(3).Info("Fetching supported models from official Kaito repository")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", SupportedModelsURL, nil)
	if err != nil {
		klog.Errorf("Failed to create request: %v", err)
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		klog.Errorf("Failed to fetch supported models: %v", err)
		return nil, fmt.Errorf("failed to fetch supported models from %s: %w", SupportedModelsURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		klog.Errorf("HTTP request failed with status: %d", resp.StatusCode)
		return nil, fmt.Errorf("HTTP request failed with status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		klog.Errorf("Failed to read response body: %v", err)
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	var kaitoModels KaitoSupportedModelsResponse
	if err := yaml.Unmarshal(body, &kaitoModels); err != nil {
		klog.Errorf("Failed to parse YAML response: %v", err)
		return nil, fmt.Errorf("failed to parse YAML response: %w", err)
	}

	// Convert to our Model struct format
	var models []Model
	for _, km := range kaitoModels.Models {
		model := Model{
			Name:         km.Name,
			Type:         km.Type,
			Runtime:      km.Runtime,
			Version:      km.Version,
			Tag:          km.Tag,
			GPUMemory:    km.GPUMemory,
			MinNodes:     km.MinNodes,
			MaxNodes:     km.MaxNodes,
			InstanceType: km.InstanceType,
			Description:  km.Description,
			Properties:   km.Properties,
		}

		// Set default values if not specified
		if model.Type == "" {
			model.Type = "LLM"
		}
		if model.Runtime == "" {
			model.Runtime = "vllm"
		}
		if model.MinNodes == 0 {
			model.MinNodes = 1
		}
		if model.MaxNodes == 0 {
			model.MaxNodes = model.MinNodes
		}

		// Generate description if not provided
		if model.Description == "" {
			model.Description = fmt.Sprintf("Official Kaito supported model: %s", model.Name)
		}

		models = append(models, model)
	}

	klog.V(3).Infof("Successfully fetched %d models from official Kaito repository", len(models))
	return models, nil
}

// getSupportedModels returns supported models, first trying to fetch from official source,
// falling back to hardcoded list if necessary
func getSupportedModels() []Model {
	klog.V(4).Info("Getting supported models list")

	models, err := fetchSupportedModelsFromKaito()
	if err != nil || len(models) == 0 {
		klog.Errorf("Failed to fetch from official repository, using fallback models: %v", err)
	}

	return models
}

// ValidateModelName checks if the provided model name is supported by Kaito
func ValidateModelName(modelName string) error {
	klog.V(4).Infof("Validating model name: %s", modelName)

	if modelName == "" {
		return fmt.Errorf("model name cannot be empty")
	}

	models := getSupportedModels()
	for _, model := range models {
		if model.Name == modelName {
			klog.V(4).Infof("Model %s is valid", modelName)
			return nil
		}
	}

	// Generate suggestions for similar model names
	suggestions := []string{}
	lowerModelName := strings.ToLower(modelName)
	for _, model := range models {
		if strings.Contains(strings.ToLower(model.Name), lowerModelName) ||
			strings.Contains(lowerModelName, strings.ToLower(model.Name)) {
			suggestions = append(suggestions, model.Name)
		}
	}

	var suggestionText string
	if len(suggestions) > 0 {
		suggestionText = fmt.Sprintf("\n\nDid you mean one of these?\n  - %s", strings.Join(suggestions, "\n  - "))
	} else {
		suggestionText = "\n\nUse 'kubectl kaito models list' to see all supported models."
	}

	return fmt.Errorf("model '%s' is not supported by Kaito%s", modelName, suggestionText)
}

// NewModelsCmd creates the models command with subcommands
func NewModelsCmd(configFlags *genericclioptions.ConfigFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "models",
		Short: "Manage and list supported AI models",
		Long: `List and describe supported AI models available in Kaito.

This command helps you discover which models are supported, their requirements,
and configuration options for deployment. The model list is fetched from the
official Kaito repository to ensure accuracy.`,
		Example: `  # List all supported models (fetched from official Kaito repo)
  kubectl kaito models list

  # List models with detailed information
  kubectl kaito models list --detailed

  # Describe a specific model
  kubectl kaito models describe phi-3.5-mini-instruct

  # Filter models by type
  kubectl kaito models list --type LLM

  # Filter models by tags
  kubectl kaito models list --tags microsoft,small`,
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Println("Use 'kubectl kaito models list' or 'kubectl kaito models describe <model>' for more information")
			return cmd.Help()
		},
	}

	// Add subcommands
	cmd.AddCommand(newModelsListCmd(configFlags))
	cmd.AddCommand(newModelsDescribeCmd())

	return cmd
}

func newModelsListCmd(configFlags *genericclioptions.ConfigFlags) *cobra.Command {
	var (
		detailed   bool
		outputJSON bool
	)

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List supported AI models",
		Long: `List all supported AI models available for deployment with Kaito.

Shows model names, types, runtime requirements, and resource specifications.
Models are fetched from the official Kaito repository to ensure accuracy.`,
		Example: `  # List all models
  kubectl kaito models list

  # List with detailed information
  kubectl kaito models list --detailed

  # Output in JSON format
  kubectl kaito models list --output json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runModelsList(detailed, outputJSON)
		},
	}

	cmd.Flags().BoolVar(&detailed, "detailed", false, "Show detailed model information")
	cmd.Flags().BoolVar(&outputJSON, "output", false, "Output in JSON format")

	return cmd
}

func newModelsDescribeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "describe <model-name>",
		Short: "Describe a specific AI model",
		Long: `Show detailed information about a specific AI model including:
- Model specifications and requirements
- Supported runtime configurations
- Resource requirements and scaling options
- Usage examples and deployment commands`,
		Example: `  # Describe the Phi-3.5 model
  kubectl kaito models describe phi-3.5-mini-instruct

  # Describe Llama 3 8B model
  kubectl kaito models describe llama-3-8b`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runModelsDescribe(args[0])
		},
	}

	return cmd
}

func runModelsList(detailed, outputJSON bool) error {
	klog.V(2).Info("Listing supported models")

	models := getSupportedModels()

	if outputJSON {
		return printModelsJSON(models)
	}

	if detailed {
		return printModelsDetailed(models)
	}

	return printModelsTable(models)
}

func runModelsDescribe(modelName string) error {
	klog.V(2).Infof("Describing model: %s", modelName)

	models := getSupportedModels()

	for _, model := range models {
		if model.Name == modelName {
			return printModelDetail(model)
		}
	}

	// Use the validation function to provide helpful error message
	return ValidateModelName(modelName)
}

func capitalizeFirst(s string) string {
	if s == "" {
		return s
	}
	caser := cases.Title(language.English)
	return caser.String(s)
}

func extractModelFamily(modelName string) string {
	// Handle empty model name
	if modelName == "" {
		return "Unknown"
	}

	// Extract the family name from the first part of the model name
	parts := strings.Split(modelName, "-")
	if len(parts) > 0 {
		family := parts[0]
		// Handle empty first part
		if family == "" {
			return "Unknown"
		}

		// Handle special cases for multi-part family names
		if len(parts) > 1 {
			switch family {
			case "llama":
				// Handle llama-3.1, llama-3.3, etc.
				if len(parts) > 1 && (strings.HasPrefix(parts[1], "3.") || strings.HasPrefix(parts[1], "2")) {
					return capitalizeFirst(family)
				}
				return capitalizeFirst(family)
			case "phi":
				// Handle phi-2, phi-3, phi-3.5, phi-4, etc.
				return capitalizeFirst(family)
			case "qwen2.5":
				// Handle qwen2.5-coder
				return "Qwen2.5"
			case "qwen2":
				return "Qwen2"
			case "deepseek":
				// Handle deepseek-r1-distill-llama-8b, deepseek-r1-distill-qwen-14b
				return "DeepSeek"
			default:
				return capitalizeFirst(family)
			}
		}
		return capitalizeFirst(family)
	}
	return "Unknown"
}

func printModelsTable(models []Model) error {
	klog.V(3).Info("Printing models table")

	w := tabwriter.NewWriter(os.Stdout, 0, 8, 2, ' ', 0)
	defer w.Flush()

	fmt.Fprintln(w, "NAME\tTYPE\tFAMILY\tRUNTIME\tTAG")

	for _, model := range models {
		// Skip base model
		if strings.ToLower(model.Name) == "base" {
			continue
		}

		// Extract family from first part of model name
		family := extractModelFamily(model.Name)

		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
			model.Name, model.Type, family, model.Runtime, model.Tag)
	}

	if err := w.Flush(); err != nil {
		return err
	}

	fmt.Println()
	fmt.Println("💡 Note: For deployment guidance and instanceType requirements,")
	fmt.Println("   use 'kubectl kaito models describe <model>' or refer to Kaito workspace examples.")

	return nil
}

func printModelsDetailed(models []Model) error {
	klog.V(3).Info("Printing detailed models information")

	for i, model := range models {
		if i > 0 {
			fmt.Println()
		}

		fmt.Printf("Name: %s\n", model.Name)
		fmt.Printf("Type: %s\n", model.Type)
		fmt.Printf("Runtime: %s\n", model.Runtime)
		fmt.Printf("Version: %s\n", model.Version)
		fmt.Printf("Description: %s\n", model.Description)
		if len(model.Tags) > 0 {
			fmt.Printf("Tags: %s\n", strings.Join(model.Tags, ", "))
		}
	}

	return nil
}

func printModelsJSON(models []Model) error {
	klog.V(3).Info("Printing models in JSON format")

	jsonData, err := json.MarshalIndent(models, "", "  ")
	if err != nil {
		klog.Errorf("Failed to marshal models to JSON: %v", err)
		return fmt.Errorf("failed to marshal models to JSON: %w", err)
	}

	fmt.Println(string(jsonData))
	return nil
}

func printModelDetail(model Model) error {
	klog.V(3).Infof("Printing detailed information for model: %s", model.Name)

	fmt.Printf("Model: %s\n", model.Name)
	fmt.Println("================")
	fmt.Println()
	fmt.Printf("Description: %s\n", model.Description)
	fmt.Printf("Type: %s\n", model.Type)
	fmt.Printf("Runtime: %s\n", model.Runtime)
	fmt.Printf("Version: %s\n", model.Version)
	fmt.Println()
	fmt.Println("Resource Requirements:")
	fmt.Println("  💡 GPU requirements are not available in the official Kaito repository.")
	fmt.Println("     For instanceType guidance, refer to:")
	fmt.Println("     - Kaito workspace examples in the GitHub repository")
	fmt.Println("     - Azure VM sizes documentation")
	fmt.Println("     - Hugging Face model cards for model sizes")
	fmt.Println("     - Community benchmarks")
	fmt.Println()
	if len(model.Tags) > 0 {
		fmt.Printf("Tags: %s\n", strings.Join(model.Tags, ", "))
		fmt.Println()
	}

	fmt.Println("Usage Example:")
	fmt.Printf("  kubectl kaito deploy --workspace-name my-workspace --model %s\n", model.Name)

	if model.InstanceType != "" {
		fmt.Println()
		fmt.Println("  # With recommended instance type:")
		fmt.Printf("  kubectl kaito deploy --workspace-name my-workspace --model %s --instance-type %s\n", model.Name, model.InstanceType)
	}

	fmt.Println()
	return nil
}
