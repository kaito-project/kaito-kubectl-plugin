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
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"
)

// ChatOptions holds the options for the chat command
type ChatOptions struct {
	configFlags *genericclioptions.ConfigFlags

	WorkspaceName string
	Namespace     string
	Temperature   float64
	MaxTokens     int
	TopP          float64
}

// NewChatCmd creates the chat command
func NewChatCmd(configFlags *genericclioptions.ConfigFlags) *cobra.Command {
	o := &ChatOptions{
		configFlags: configFlags,
		Temperature: 0.7,
		MaxTokens:   1024,
		TopP:        0.9,
	}

	cmd := &cobra.Command{
		Use:   "chat",
		Short: "Interactive chat with deployed AI models",
		Long: `Start an interactive chat session with a deployed Kaito workspace model.

This command provides a chat interface to interact with deployed models using
OpenAI-compatible APIs in interactive mode.`,
		Example: `  # Start interactive chat session
  kubectl kaito chat --workspace-name my-llama

  # Configure inference parameters
  kubectl kaito chat --workspace-name my-llama --temperature 0.5 --max-tokens 512

  # Pipe input for non-interactive usage
  echo "What is AI?" | kubectl kaito chat --workspace-name my-llama`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := o.validate(); err != nil {
				klog.Errorf("Validation failed: %v", err)
				return fmt.Errorf("validation failed: %w", err)
			}
			return o.run()
		},
	}

	cmd.Flags().StringVar(&o.WorkspaceName, "workspace-name", "", "Name of the workspace (required)")
	cmd.Flags().StringVarP(&o.Namespace, "namespace", "n", "", "Kubernetes namespace")
	cmd.Flags().Float64Var(&o.Temperature, "temperature", 0.7, "Temperature for response generation (0.0-2.0)")
	cmd.Flags().IntVar(&o.MaxTokens, "max-tokens", 1024, "Maximum tokens in response")
	cmd.Flags().Float64Var(&o.TopP, "top-p", 0.9, "Top-p (nucleus sampling) parameter (0.0-1.0)")

	if err := cmd.MarkFlagRequired("workspace-name"); err != nil {
		klog.Errorf("Failed to mark workspace-name flag as required: %v", err)
	}

	return cmd
}

func (o *ChatOptions) validate() error {
	klog.V(4).Info("Validating chat options")

	if o.WorkspaceName == "" {
		return fmt.Errorf("workspace name is required")
	}
	if o.Temperature < 0.0 || o.Temperature > 2.0 {
		return fmt.Errorf("temperature must be between 0.0 and 2.0")
	}
	if o.TopP < 0.0 || o.TopP > 1.0 {
		return fmt.Errorf("top-p must be between 0.0 and 1.0")
	}
	if o.MaxTokens <= 0 {
		return fmt.Errorf("max-tokens must be greater than 0")
	}

	klog.V(4).Info("Chat validation completed successfully")
	return nil
}

func (o *ChatOptions) run() error {
	klog.V(2).Infof("Starting chat with workspace: %s", o.WorkspaceName)

	// Get namespace
	if o.Namespace == "" {
		if ns, _, err := o.configFlags.ToRawKubeConfigLoader().Namespace(); err == nil && ns != "" {
			o.Namespace = ns
		} else {
			klog.V(4).Info("No namespace specified, using 'default'")
			o.Namespace = "default"
		}
	}

	// Get REST config
	config, err := o.configFlags.ToRESTConfig()
	if err != nil {
		return fmt.Errorf("failed to get REST config: %w", err)
	}

	// Create clients
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return fmt.Errorf("failed to create kubernetes client: %w", err)
	}

	// Get the endpoint URL
	endpoint, err := o.getInferenceEndpoint(context.TODO(), clientset)
	if err != nil {
		return err
	}

	klog.V(3).Infof("Using endpoint: %s", endpoint)

	// Get model name for display
	modelName, err := o.getModelName(config)
	if err != nil {
		klog.V(4).Infof("Could not get model name: %v", err)
		modelName = "Unknown"
	}

	// Start interactive session
	return o.startInteractiveSession(endpoint, modelName)
}

func (o *ChatOptions) getInferenceEndpoint(ctx context.Context, clientset kubernetes.Interface) (string, error) {
	klog.V(3).Info("Getting inference endpoint")

	// Get the service for the workspace (service name equals workspace name)
	svc, err := clientset.CoreV1().Services(o.Namespace).Get(ctx, o.WorkspaceName, metav1.GetOptions{})
	if err != nil {
		return "", fmt.Errorf("failed to get service for workspace %s: %w", o.WorkspaceName, err)
	}

	if svc.Spec.ClusterIP == "" || svc.Spec.ClusterIP == "None" {
		return "", fmt.Errorf("service %s has no cluster IP", o.WorkspaceName)
	}

	var baseEndpoint string

	// Try cluster-internal endpoint first (if running inside cluster)
	clusterEndpoint := fmt.Sprintf("http://%s.%s.svc.cluster.local:80", o.WorkspaceName, o.Namespace)
	if o.canAccessClusterEndpoint(clusterEndpoint) {
		baseEndpoint = clusterEndpoint
		klog.V(3).Infof("Using cluster-internal endpoint: %s", baseEndpoint)
	} else {
		// Use Kubernetes API Proxy - works from anywhere kubectl works!
		apiProxyEndpoint, err := o.getAPIProxyEndpoint(clientset)
		if err != nil {
			return "", fmt.Errorf("failed to get API proxy endpoint: %w", err)
		}
		baseEndpoint = apiProxyEndpoint
		klog.V(3).Infof("Using Kubernetes API proxy endpoint: %s", baseEndpoint)
	}

	// Return OpenAI-compatible chat endpoint
	chatEndpoint := fmt.Sprintf("%s/v1/chat/completions", baseEndpoint)
	klog.V(3).Infof("Chat endpoint: %s", chatEndpoint)
	return chatEndpoint, nil
}

// canAccessClusterEndpoint checks if we can reach the cluster-internal endpoint
func (o *ChatOptions) canAccessClusterEndpoint(endpoint string) bool {
	// Try to resolve the cluster DNS name
	_, err := net.LookupHost(strings.TrimPrefix(strings.TrimPrefix(endpoint, "http://"), "https://"))
	return err == nil
}

func (o *ChatOptions) getModelName(config interface{}) (string, error) {
	klog.V(4).Info("Getting model name from workspace")

	workspace, err := o.getWorkspace()
	if err != nil {
		return "", err
	}

	// Try to get model name from different possible locations in the workspace spec
	if modelName := o.extractModelFromSpec(workspace); modelName != "" {
		return modelName, nil
	}

	if modelName := o.extractModelFromInference(workspace); modelName != "" {
		return modelName, nil
	}

	if modelName := o.extractModelFromInferenceModel(workspace); modelName != "" {
		return modelName, nil
	}

	return "Unknown", nil
}

func (o *ChatOptions) getWorkspace() (*unstructured.Unstructured, error) {
	// Get REST config
	restConfig, err := o.configFlags.ToRESTConfig()
	if err != nil {
		klog.Errorf("Failed to get REST config: %v", err)
		return nil, fmt.Errorf("failed to get REST config: %w", err)
	}

	// Create dynamic client
	dynamicClient, err := dynamic.NewForConfig(restConfig)
	if err != nil {
		klog.Errorf("Failed to create dynamic client: %v", err)
		return nil, fmt.Errorf("failed to create dynamic client: %w", err)
	}

	gvr := schema.GroupVersionResource{
		Group:    "kaito.sh",
		Version:  "v1beta1",
		Resource: "workspaces",
	}

	workspace, err := dynamicClient.Resource(gvr).Namespace(o.Namespace).Get(
		context.TODO(),
		o.WorkspaceName,
		metav1.GetOptions{},
	)
	if err != nil {
		klog.Errorf("Failed to get workspace %s: %v", o.WorkspaceName, err)
		return nil, fmt.Errorf("failed to get workspace %s: %w", o.WorkspaceName, err)
	}

	return workspace, nil
}

func (o *ChatOptions) extractModelFromSpec(workspace *unstructured.Unstructured) string {
	// Try: spec.inference.preset.name
	return o.extractStringFromPath(workspace.Object, []string{"spec", "inference", "preset", "name"})
}

func (o *ChatOptions) extractModelFromInference(workspace *unstructured.Unstructured) string {
	// Try: top-level inference.preset.name (for newer workspace structure)
	return o.extractStringFromPath(workspace.Object, []string{"inference", "preset", "name"})
}

func (o *ChatOptions) extractModelFromInferenceModel(workspace *unstructured.Unstructured) string {
	// Try: Check if there's a model field directly in inference
	return o.extractStringFromPath(workspace.Object, []string{"inference", "model"})
}

func (o *ChatOptions) extractStringFromPath(obj map[string]interface{}, path []string) string {
	if len(path) == 0 || obj == nil {
		return ""
	}

	current := obj
	for i, key := range path {
		value, exists := current[key]
		if !exists {
			return ""
		}

		// Handle the final element in the path
		if i == len(path)-1 {
			return o.convertToString(value)
		}

		// Handle intermediate elements - must be maps
		if nextMap, ok := value.(map[string]interface{}); ok {
			current = nextMap
		} else {
			return ""
		}
	}

	return ""
}

// convertToString safely converts various types to string
func (o *ChatOptions) convertToString(value interface{}) string {
	switch v := value.(type) {
	case string:
		return v
	case *string:
		if v != nil {
			return *v
		}
	case fmt.Stringer:
		return v.String()
	default:
		// For other types, use fmt.Sprintf for safe conversion
		if str := fmt.Sprintf("%v", v); str != "<nil>" && str != "" {
			return str
		}
	}
	return ""
}

func (o *ChatOptions) startInteractiveSession(endpoint, modelName string) error {
	klog.V(2).Info("Starting interactive chat session")

	fmt.Printf("Connected to workspace: %s (model: %s)\n", o.WorkspaceName, modelName)
	fmt.Println("Type /help for commands or /quit to exit.")
	fmt.Println()

	scanner := bufio.NewScanner(os.Stdin)

	for {
		fmt.Print(">>> ")
		if !scanner.Scan() {
			if err := scanner.Err(); err != nil {
				fmt.Println("\nChat session ended.")
				return nil
			}
			klog.Errorf("Error reading input: %v", scanner.Err())
			return fmt.Errorf("error reading input: %w", scanner.Err())
		}

		input := strings.TrimSpace(scanner.Text())

		// Handle commands
		if strings.HasPrefix(input, "/") {
			if o.handleCommand(input, modelName) {
				return nil // Exit command
			}
			continue
		}

		// Skip empty input
		if input == "" {
			continue
		}

		// Send message and get response
		response, err := o.sendMessage(endpoint, input)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			continue
		}

		fmt.Println(response)
		fmt.Println()
	}
}

func (o *ChatOptions) handleCommand(command, modelName string) bool {
	klog.V(4).Infof("Handling command: %s", command)

	parts := strings.Fields(command)
	if len(parts) == 0 {
		return false
	}

	switch parts[0] {
	case "/help":
		fmt.Println("Available commands:")
		fmt.Println("  /help        - Show this help message")
		fmt.Println("  /quit        - Exit the chat session")
		fmt.Println("  /clear       - Clear the conversation history")
		fmt.Println("  /model       - Show current model information")
		fmt.Println("  /params      - Show current inference parameters")
		fmt.Println("  /set <param> <value> - Set inference parameter (temperature, max_tokens, etc.)")
		fmt.Println()

	case "/quit", "/exit":
		fmt.Println("Chat session ended.")
		return true

	case "/clear":
		fmt.Print("\033[2J\033[H") // Clear screen
		fmt.Printf("Connected to workspace: %s (model: %s)\n", o.WorkspaceName, modelName)
		fmt.Println("Type /help for commands or /quit to exit.")
		fmt.Println()

	case "/model":
		fmt.Printf("Current model: %s\n", modelName)
		fmt.Printf("Workspace: %s\n", o.WorkspaceName)
		fmt.Printf("Namespace: %s\n", o.Namespace)
		fmt.Println()

	case "/params":
		fmt.Println("Current inference parameters:")
		fmt.Printf("  Temperature: %.1f\n", o.Temperature)
		fmt.Printf("  Max tokens: %d\n", o.MaxTokens)
		fmt.Printf("  Top-p: %.1f\n", o.TopP)
		fmt.Println()

	case "/set":
		if len(parts) < 3 {
			fmt.Println("Usage: /set <parameter> <value>")
			fmt.Println("Available parameters: temperature, max_tokens, top_p")
			fmt.Println()
			return false
		}
		o.setParameter(parts[1], parts[2])

	default:
		fmt.Printf("Unknown command: %s\n", parts[0])
		fmt.Println("Type /help for available commands.")
		fmt.Println()
	}

	return false
}

func (o *ChatOptions) setParameter(param, value string) {
	klog.V(4).Infof("Setting parameter %s to %s", param, value)

	switch param {
	case "temperature":
		if temp, err := strconv.ParseFloat(value, 64); err == nil && temp >= 0.0 && temp <= 2.0 {
			o.Temperature = temp
			fmt.Printf("Temperature set to %.1f\n", temp)
		} else {
			fmt.Println("Invalid temperature value. Must be between 0.0 and 2.0")
		}

	case "max_tokens":
		if tokens, err := strconv.Atoi(value); err == nil && tokens > 0 {
			o.MaxTokens = tokens
			fmt.Printf("Max tokens set to %d\n", tokens)
		} else {
			fmt.Println("Invalid max_tokens value. Must be a positive integer")
		}

	case "top_p":
		if topP, err := strconv.ParseFloat(value, 64); err == nil && topP >= 0.0 && topP <= 1.0 {
			o.TopP = topP
			fmt.Printf("Top-p set to %.1f\n", topP)
		} else {
			fmt.Println("Invalid top_p value. Must be between 0.0 and 1.0")
		}

	default:
		fmt.Printf("Unknown parameter: %s\n", param)
		fmt.Println("Available parameters: temperature, max_tokens, top_p")
	}
	fmt.Println()
}

func (o *ChatOptions) sendMessage(endpoint, message string) (string, error) {
	klog.V(4).Infof("Sending message to endpoint: %s", endpoint)

	payload := o.buildRequestPayload(message)
	jsonData, err := json.Marshal(payload)
	if err != nil {
		klog.Errorf("Failed to marshal request: %v", err)
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	response, err := o.makeHTTPRequest(endpoint, jsonData)
	if err != nil {
		return "", err
	}

	return o.extractMessageContent(response)
}

func (o *ChatOptions) buildRequestPayload(message string) map[string]interface{} {
	payload := map[string]interface{}{
		"messages": []map[string]string{
			{
				"role":    "user",
				"content": message,
			},
		},
		"temperature": o.Temperature,
		"max_tokens":  o.MaxTokens,
		"top_p":       o.TopP,
	}

	return payload
}

func (o *ChatOptions) makeHTTPRequest(endpoint string, jsonData []byte) (map[string]interface{}, error) {
	client, err := o.createHTTPClient(endpoint)
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP client: %w", err)
	}

	resp, err := client.Post(endpoint, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		klog.Errorf("Failed to send request: %v", err)
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err == nil && resp.StatusCode != http.StatusOK {
		klog.Errorf("API request failed with status %d: %s", resp.StatusCode, string(body))
		return nil, fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(body))
	}

	if err != nil {
		klog.Errorf("Failed to read response: %v", err)
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(body, &response); err != nil {
		klog.Errorf("Failed to parse response: %v", err)
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return response, nil
}

// createHTTPClient creates an HTTP client with proper authentication for API proxy endpoints
func (o *ChatOptions) createHTTPClient(endpoint string) (*http.Client, error) {
	client := &http.Client{Timeout: 30 * time.Second}

	// If this is an API proxy endpoint, we need to add authentication
	if strings.Contains(endpoint, "/api/v1/namespaces/") {
		config, err := o.configFlags.ToRESTConfig()
		if err != nil {
			return nil, fmt.Errorf("failed to get REST config: %w", err)
		}

		// Set up the transport with authentication
		transport, err := o.createAuthenticatedTransport(config)
		if err != nil {
			return nil, fmt.Errorf("failed to create authenticated transport: %w", err)
		}
		client.Transport = transport
	}

	return client, nil
}

// createAuthenticatedTransport creates an HTTP transport with Kubernetes authentication
func (o *ChatOptions) createAuthenticatedTransport(config *rest.Config) (http.RoundTripper, error) {
	// Use the existing REST config's transport
	transport, err := rest.TransportFor(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create transport: %w", err)
	}
	return transport, nil
}

func (o *ChatOptions) extractMessageContent(response map[string]interface{}) (string, error) {
	choices, ok := response["choices"].([]interface{})
	if !ok || len(choices) == 0 {
		klog.Error("Unexpected response format: no choices")
		return "", fmt.Errorf("unexpected response format: no choices")
	}

	choice, ok := choices[0].(map[string]interface{})
	if !ok {
		klog.Error("Unexpected response format: invalid choice")
		return "", fmt.Errorf("unexpected response format: invalid choice")
	}

	message, ok := choice["message"].(map[string]interface{})
	if !ok {
		klog.Error("Unexpected response format: no message")
		return "", fmt.Errorf("unexpected response format: no message")
	}

	content, ok := message["content"].(string)
	if !ok {
		klog.Error("Unexpected response format: no content")
		return "", fmt.Errorf("unexpected response format: no content")
	}

	return strings.TrimSpace(content), nil
}

// getAPIProxyEndpoint constructs the Kubernetes API proxy endpoint for the service
func (o *ChatOptions) getAPIProxyEndpoint(clientset kubernetes.Interface) (string, error) {
	// Get the REST config to build the API server URL
	config, err := o.configFlags.ToRESTConfig()
	if err != nil {
		return "", fmt.Errorf("failed to get REST config: %w", err)
	}

	// Build the API proxy URL
	// Format: https://{api-server}/api/v1/namespaces/{namespace}/services/{service-name}:{port}/proxy
	namespace := o.Namespace
	if namespace == "" {
		namespace = "default"
	}

	apiProxyURL := fmt.Sprintf("%s/api/v1/namespaces/%s/services/%s:80/proxy",
		strings.TrimSuffix(config.Host, "/"), namespace, o.WorkspaceName)

	klog.V(3).Infof("Constructed API proxy URL: %s", apiProxyURL)
	return apiProxyURL, nil
}
