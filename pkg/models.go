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
const SupportedModelsURL = "https://raw.githubusercontent.com/kaito-project/kaito/main/presets/workspace/models/model_catalog.yaml"

// Model represents a supported AI model from the official Kaito repository
type Model struct {
	Name            string   `json:"name" yaml:"name"`
	Description     string   `json:"description" yaml:"description"`
	License         string   `json:"license" yaml:"license"`
	PipelineTag     string   `json:"pipeline_tag" yaml:"pipelineTag"`
	ModelFileSize   string   `json:"model_file_size" yaml:"modelFileSize"`
	Architectures   []string `json:"architectures" yaml:"architectures"`
	BaseModel       []string `json:"base_model,omitempty" yaml:"baseModel,omitempty"`
	QuantMethod     string   `json:"quant_method,omitempty" yaml:"quantMethod,omitempty"`
	ModelTokenLimit int      `json:"model_token_limit" yaml:"modelTokenLimit"`
	HiddenSize      int      `json:"hidden_size" yaml:"hiddenSize"`
	NumHiddenLayers int      `json:"num_hidden_layers" yaml:"numHiddenLayers"`
	NumAttHeads     int      `json:"num_attention_heads" yaml:"numAttentionHeads"`
	NumKVHeads      int      `json:"num_key_value_heads" yaml:"numKeyValueHeads"`
	HeadDim         int      `json:"head_dim,omitempty" yaml:"headDim,omitempty"`
	QuantBits       int      `json:"quant_bits,omitempty" yaml:"quantBits,omitempty"`
	KVLoraRank      int      `json:"kv_lora_rank,omitempty" yaml:"kvLoraRank,omitempty"`
	QKRopeHeadDim   int      `json:"qk_rope_head_dim,omitempty" yaml:"qkRopeHeadDim,omitempty"`
	LoadFormat      string   `json:"load_format,omitempty" yaml:"loadFormat,omitempty"`
	ConfigFormat    string   `json:"config_format,omitempty" yaml:"configFormat,omitempty"`
	TokenizerMode   string   `json:"tokenizer_mode,omitempty" yaml:"tokenizerMode,omitempty"`
}

// KaitoModelCatalogResponse represents the structure of the official model_catalog.yaml
type KaitoModelCatalogResponse struct {
	Models []struct {
		Name            string   `yaml:"name"`
		Description     string   `yaml:"description,omitempty"`
		License         string   `yaml:"license,omitempty"`
		PipelineTag     string   `yaml:"pipelineTag,omitempty"`
		ModelFileSize   string   `yaml:"modelFileSize,omitempty"`
		Architectures   []string `yaml:"architectures,omitempty"`
		BaseModel       []string `yaml:"baseModel,omitempty"`
		QuantMethod     string   `yaml:"quantMethod,omitempty"`
		ModelTokenLimit int      `yaml:"modelTokenLimit,omitempty"`
		HiddenSize      int      `yaml:"hiddenSize,omitempty"`
		NumHiddenLayers int      `yaml:"numHiddenLayers,omitempty"`
		NumAttHeads     int      `yaml:"numAttentionHeads,omitempty"`
		NumKVHeads      int      `yaml:"numKeyValueHeads,omitempty"`
		HeadDim         int      `yaml:"headDim,omitempty"`
		QuantBits       int      `yaml:"quantBits,omitempty"`
		KVLoraRank      int      `yaml:"kvLoraRank,omitempty"`
		QKRopeHeadDim   int      `yaml:"qkRopeHeadDim,omitempty"`
		LoadFormat      string   `yaml:"loadFormat,omitempty"`
		ConfigFormat    string   `yaml:"configFormat,omitempty"`
		TokenizerMode   string   `yaml:"tokenizerMode,omitempty"`
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

	var catalog KaitoModelCatalogResponse
	if err := yaml.Unmarshal(body, &catalog); err != nil {
		klog.Errorf("Failed to parse YAML response: %v", err)
		return nil, fmt.Errorf("failed to parse YAML response: %w", err)
	}

	// Convert to our Model struct format
	var models []Model
	for _, km := range catalog.Models {
		model := Model{
			Name:            km.Name,
			Description:     km.Description,
			License:         km.License,
			PipelineTag:     km.PipelineTag,
			ModelFileSize:   km.ModelFileSize,
			Architectures:   km.Architectures,
			BaseModel:       km.BaseModel,
			QuantMethod:     km.QuantMethod,
			ModelTokenLimit: km.ModelTokenLimit,
			HiddenSize:      km.HiddenSize,
			NumHiddenLayers: km.NumHiddenLayers,
			NumAttHeads:     km.NumAttHeads,
			NumKVHeads:      km.NumKVHeads,
			HeadDim:         km.HeadDim,
			QuantBits:       km.QuantBits,
			KVLoraRank:      km.KVLoraRank,
			QKRopeHeadDim:   km.QKRopeHeadDim,
			LoadFormat:      km.LoadFormat,
			ConfigFormat:    km.ConfigFormat,
			TokenizerMode:   km.TokenizerMode,
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

// IsHuggingFaceModel checks if the model name is a HuggingFace model ID (contains '/')
func IsHuggingFaceModel(modelName string) bool {
	return strings.Contains(modelName, "/")
}

// ValidateModelName checks if the provided model name is supported by Kaito.
// Accepts both models from the Kaito catalog and any valid HuggingFace model ID (org/model-name).
func ValidateModelName(modelName string) error {
	klog.V(4).Infof("Validating model name: %s", modelName)

	if modelName == "" {
		return fmt.Errorf("model name cannot be empty")
	}

	// HuggingFace model IDs (containing '/') are always accepted.
	// Kaito dynamically generates presets for any valid HuggingFace model.
	if IsHuggingFaceModel(modelName) {
		parts := strings.SplitN(modelName, "/", 2)
		if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
			return fmt.Errorf("invalid HuggingFace model ID '%s': expected format 'org/model-name'", modelName)
		}
		klog.V(4).Infof("Model %s is a valid HuggingFace model ID", modelName)
		return nil
	}

	// Non-HuggingFace style names are not valid since all catalog models now use org/model format
	models := getSupportedModels()

	// Generate suggestions for similar model names
	suggestions := []string{}
	lowerModelName := strings.ToLower(modelName)
	for _, model := range models {
		// Check if the model name part (after /) matches
		parts := strings.SplitN(model.Name, "/", 2)
		modelShortName := model.Name
		if len(parts) == 2 {
			modelShortName = parts[1]
		}
		if strings.Contains(strings.ToLower(modelShortName), lowerModelName) ||
			strings.Contains(lowerModelName, strings.ToLower(modelShortName)) {
			suggestions = append(suggestions, model.Name)
		}
	}

	var suggestionText string
	if len(suggestions) > 0 {
		suggestionText = fmt.Sprintf("\n\nDid you mean one of these?\n  - %s", strings.Join(suggestions, "\n  - "))
	} else {
		suggestionText = "\n\nUse 'kubectl kaito models list' to see all supported models."
	}
	suggestionText += "\n\n💡 Use the full HuggingFace model ID format (e.g., 'microsoft/Phi-3.5-mini-instruct')."

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
  kubectl kaito models describe microsoft/Phi-3.5-mini-instruct`,
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
  kubectl kaito models describe microsoft/Phi-3.5-mini-instruct

  # Describe Llama 3.1 8B model
  kubectl kaito models describe meta-llama/Llama-3.1-8B-Instruct`,
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

	// Check if the model is in the catalog
	for _, model := range models {
		if model.Name == modelName {
			return printModelDetail(model)
		}
	}

	// If it's a valid HuggingFace model ID but not in catalog, show generic info
	if IsHuggingFaceModel(modelName) {
		parts := strings.SplitN(modelName, "/", 2)
		if len(parts) == 2 && parts[0] != "" && parts[1] != "" {
			fmt.Printf("Model: %s (HuggingFace)\n", modelName)
			fmt.Println("================")
			fmt.Println()
			fmt.Println("This model is not in the Kaito catalog but can still be deployed.")
			fmt.Println("Kaito will dynamically generate a preset configuration for this model at deployment time.")
			fmt.Println()
			fmt.Println("Requirements:")
			fmt.Println("  - The model architecture must be supported by vLLM")
			fmt.Println("  - A HuggingFace access token may be required (use --model-access-secret)")
			fmt.Println()
			fmt.Println("Usage Example:")
			fmt.Printf("  kubectl kaito deploy --workspace-name my-workspace --model %s --model-access-secret hf-token\n", modelName)
			fmt.Println()
			fmt.Printf("  For model details, visit: https://huggingface.co/%s\n", modelName)
			fmt.Println()
			return nil
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
	if modelName == "" {
		return "Unknown"
	}

	// For HuggingFace-style names (org/model), extract family from the model part
	if strings.Contains(modelName, "/") {
		parts := strings.SplitN(modelName, "/", 2)
		if len(parts) == 2 && parts[1] != "" {
			modelName = parts[1]
		} else {
			return "Unknown"
		}
	}

	// Extract the family name from the first part of the model name
	parts := strings.Split(modelName, "-")
	if len(parts) == 0 || parts[0] == "" {
		return "Unknown"
	}

	family := parts[0]
	lowerFamily := strings.ToLower(family)

	// Families with fixed canonical names
	switch lowerFamily {
	case "qwen2.5":
		return "Qwen2.5"
	case "qwen2":
		return "Qwen2"
	case "deepseek":
		return "DeepSeek"
	case "nvidia":
		if len(parts) > 1 {
			return capitalizeFirst(parts[1])
		}
	}

	// Families that include the version suffix (e.g., "Llama-3.1", "Phi-4", "Gemma-3")
	versionedFamilies := map[string]bool{
		"llama": true, "phi": true, "ministral": true, "gemma": true,
	}
	if len(parts) > 1 && versionedFamilies[lowerFamily] {
		return capitalizeFirst(family + "-" + parts[1])
	}

	return capitalizeFirst(family)
}

func printModelsTable(models []Model) error {
	klog.V(3).Info("Printing models table")

	w := tabwriter.NewWriter(os.Stdout, 0, 8, 2, ' ', 0)
	defer w.Flush()

	fmt.Fprintln(w, "NAME\tFAMILY\tPIPELINE\tLICENSE\tSIZE")

	for _, model := range models {
		// Extract family from org/model-name format
		family := extractModelFamily(model.Name)

		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
			model.Name, family, model.PipelineTag, model.License, model.ModelFileSize)
	}

	if err := w.Flush(); err != nil {
		return err
	}

	fmt.Println()
	fmt.Println("💡 You can also deploy any model from HuggingFace by using its model ID:")
	fmt.Println("   kubectl kaito deploy --workspace-name my-workspace --model Qwen/Qwen3-4B-Instruct-2507 --model-access-secret hf-token")
	fmt.Println()
	fmt.Println("   For model details, use 'kubectl kaito models describe <model>'.")

	return nil
}

func printModelsDetailed(models []Model) error {
	klog.V(3).Info("Printing detailed models information")

	for i, model := range models {
		if i > 0 {
			fmt.Println()
		}

		fmt.Printf("Name: %s\n", model.Name)
		fmt.Printf("License: %s\n", model.License)
		fmt.Printf("Pipeline: %s\n", model.PipelineTag)
		fmt.Printf("Size: %s\n", model.ModelFileSize)
		if len(model.Architectures) > 0 {
			fmt.Printf("Architectures: %s\n", strings.Join(model.Architectures, ", "))
		}
		if model.ModelTokenLimit > 0 {
			fmt.Printf("Token Limit: %d\n", model.ModelTokenLimit)
		}
		if model.QuantMethod != "" {
			fmt.Printf("Quantization: %s\n", model.QuantMethod)
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
	if model.Description != "" {
		fmt.Printf("Description: %s\n", model.Description)
	}
	fmt.Printf("License: %s\n", model.License)
	fmt.Printf("Pipeline: %s\n", model.PipelineTag)
	fmt.Printf("Model File Size: %s\n", model.ModelFileSize)
	fmt.Println()
	if len(model.Architectures) > 0 {
		fmt.Printf("Architectures: %s\n", strings.Join(model.Architectures, ", "))
	}
	if model.ModelTokenLimit > 0 {
		fmt.Printf("Token Limit: %d\n", model.ModelTokenLimit)
	}
	fmt.Println()
	fmt.Println("Model Architecture:")
	if model.HiddenSize > 0 {
		fmt.Printf("  Hidden Size: %d\n", model.HiddenSize)
	}
	if model.NumHiddenLayers > 0 {
		fmt.Printf("  Hidden Layers: %d\n", model.NumHiddenLayers)
	}
	if model.NumAttHeads > 0 {
		fmt.Printf("  Attention Heads: %d\n", model.NumAttHeads)
	}
	if model.NumKVHeads > 0 {
		fmt.Printf("  KV Heads: %d\n", model.NumKVHeads)
	}
	if model.HeadDim > 0 {
		fmt.Printf("  Head Dimension: %d\n", model.HeadDim)
	}
	fmt.Println()
	if model.QuantMethod != "" {
		fmt.Printf("Quantization: %s\n", model.QuantMethod)
		if model.QuantBits > 0 {
			fmt.Printf("Quant Bits: %d\n", model.QuantBits)
		}
		fmt.Println()
	}
	if len(model.BaseModel) > 0 {
		fmt.Printf("Base Model: %s\n", strings.Join(model.BaseModel, ", "))
		fmt.Println()
	}

	fmt.Println("Usage Example:")
	fmt.Printf("  kubectl kaito deploy --workspace-name my-workspace --model %s\n", model.Name)
	fmt.Println()
	return nil
}
