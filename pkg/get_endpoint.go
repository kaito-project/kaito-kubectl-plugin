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
	"net"
	"strings"

	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
)

// EndpointInfo represents an available endpoint
type EndpointInfo struct {
	URL         string `json:"url"`
	Type        string `json:"type"`
	Access      string `json:"access"`
	Description string `json:"description"`
}

// GetEndpointOptions holds the options for the get-endpoint command
type GetEndpointOptions struct {
	configFlags   *genericclioptions.ConfigFlags
	WorkspaceName string
	Namespace     string
	Format        string
}

// NewGetEndpointCmd creates the get-endpoint command
func NewGetEndpointCmd(configFlags *genericclioptions.ConfigFlags) *cobra.Command {
	o := &GetEndpointOptions{
		configFlags: configFlags,
	}

	cmd := &cobra.Command{
		Use:   "get-endpoint",
		Short: "Get inference endpoints for a Kaito workspace",
		Long: `Get the inference endpoint URL for a deployed Kaito workspace.

This command retrieves the service endpoint that can be used to send inference
requests to the deployed model. The endpoint supports OpenAI-compatible APIs.`,
		Example: `  # Get endpoint URL for a workspace
  kubectl kaito get-endpoint --workspace-name my-workspace

  # Get endpoint in JSON format with metadata
  kubectl kaito get-endpoint --workspace-name my-workspace --format json

  # Get all available endpoints
  kubectl kaito get-endpoint --workspace-name my-workspace --format json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := o.validate(); err != nil {
				return err
			}
			return o.run()
		},
	}

	cmd.Flags().StringVar(&o.WorkspaceName, "workspace-name", "", "Name of the workspace (required)")
	cmd.Flags().StringVarP(&o.Namespace, "namespace", "n", "", "Kubernetes namespace")
	cmd.Flags().StringVar(&o.Format, "format", "url", "Output format: url or json")

	if err := cmd.MarkFlagRequired("workspace-name"); err != nil {
		klog.Errorf("Failed to mark workspace-name flag as required: %v", err)
	}

	return cmd
}

func (o *GetEndpointOptions) validate() error {
	klog.V(4).Info("Validating get-endpoint options")

	if o.WorkspaceName == "" {
		return fmt.Errorf("workspace name is required")
	}
	if o.Format != "url" && o.Format != "json" {
		return fmt.Errorf("format must be 'url' or 'json'")
	}

	klog.V(4).Info("Get-endpoint validation completed successfully")
	return nil
}

func (o *GetEndpointOptions) run() error {
	klog.V(2).Infof("Getting endpoint for workspace: %s", o.WorkspaceName)

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

	// Create dynamic client
	dynamicClient, err := dynamic.NewForConfig(config)
	if err != nil {
		return fmt.Errorf("failed to create dynamic client: %w", err)
	}

	// Create kubernetes client
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return fmt.Errorf("failed to create kubernetes client: %w", err)
	}

	// Check workspace status first
	if err2 := o.checkWorkspaceReady(dynamicClient); err2 != nil {
		return err
	}

	// Get all available endpoints
	endpoints, err := o.getAllEndpoints(context.TODO(), clientset)
	if err != nil {
		return err
	}

	// Output the result
	if o.Format == "json" {
		output := map[string]interface{}{
			"workspace": o.WorkspaceName,
			"namespace": o.Namespace,
			"endpoints": endpoints,
		}

		jsonOutput, err := json.MarshalIndent(output, "", "  ")
		if err != nil {
			klog.Errorf("Failed to marshal JSON: %v", err)
			return fmt.Errorf("failed to marshal JSON: %w", err)
		}
		fmt.Println(string(jsonOutput))
	} else {
		// For URL format, show the best endpoint (prefer external if available)
		if len(endpoints) == 0 {
			return fmt.Errorf("no endpoints available for workspace %s", o.WorkspaceName)
		}

		// Prefer external endpoints, fallback to first available
		for _, ep := range endpoints {
			if ep.Access == "external" {
				fmt.Println(ep.URL)
				return nil
			}
		}
		// No external endpoint, use the first one
		fmt.Println(endpoints[0].URL)
	}

	return nil
}

func (o *GetEndpointOptions) checkWorkspaceReady(dynamicClient dynamic.Interface) error {
	klog.V(3).Info("Checking workspace readiness")

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
		return fmt.Errorf("failed to get workspace %s: %w", o.WorkspaceName, err)
	}

	// Check if workspace has status
	status, found := workspace.Object["status"]
	if !found {
		return fmt.Errorf("workspace %s has no status information", o.WorkspaceName)
	}

	// Check workspace ready condition
	if !o.isWorkspaceReady(status) {
		return fmt.Errorf("workspace %s is not ready yet. Use 'kubectl kaito status --workspace-name %s' to check status", o.WorkspaceName, o.WorkspaceName)
	}

	klog.V(3).Info("Workspace is ready")
	return nil
}

func (o *GetEndpointOptions) isWorkspaceReady(status interface{}) bool {
	statusMap, ok := status.(map[string]interface{})
	if !ok {
		klog.V(6).Info("Status is not a map")
		return false
	}

	conditions, found := statusMap["conditions"]
	if !found {
		klog.V(6).Info("No conditions found in status")
		return false
	}

	conditionsList, ok := conditions.([]interface{})
	if !ok {
		klog.V(6).Info("Conditions is not a slice")
		return false
	}

	// For endpoint access, we need both ResourceReady and InferenceReady to be True
	resourceReady := false
	inferenceReady := false

	for _, condition := range conditionsList {
		condMap, ok := condition.(map[string]interface{})
		if !ok {
			continue
		}

		condType, ok := condMap["type"].(string)
		if !ok {
			continue
		}

		condStatus, ok := condMap["status"].(string)
		if !ok {
			continue
		}

		switch condType {
		case "ResourceReady":
			if condStatus == "True" {
				resourceReady = true
			}
		case "InferenceReady":
			if condStatus == "True" {
				inferenceReady = true
			}
		}
	}

	// Return true only if both resource and inference are ready
	return resourceReady && inferenceReady
}

func (o *GetEndpointOptions) getAllEndpoints(ctx context.Context, clientset kubernetes.Interface) ([]EndpointInfo, error) {
	klog.V(3).Infof("Getting all endpoints for workspace: %s", o.WorkspaceName)

	svc, err := clientset.CoreV1().Services(o.Namespace).Get(ctx, o.WorkspaceName, metav1.GetOptions{})
	if err != nil {
		klog.Errorf("Failed to get service for workspace %s: %v", o.WorkspaceName, err)
		return nil, fmt.Errorf("failed to get service for workspace %s: %v", o.WorkspaceName, err)
	}

	var endpoints []EndpointInfo

	// Check for LoadBalancer endpoint (external access)
	if lbEndpoint := o.getLoadBalancerEndpoint(svc); lbEndpoint != "" {
		endpoints = append(endpoints, EndpointInfo{
			URL:         lbEndpoint,
			Type:        "LoadBalancer",
			Access:      "external",
			Description: "Direct public access via LoadBalancer",
		})
	}

	// Always add the API proxy endpoint (works anywhere kubectl works)
	apiProxyEndpoint, err := o.getAPIProxyEndpoint()
	if err != nil {
		klog.V(3).Infof("Could not get API proxy endpoint: %v", err)
	} else {
		endpoints = append(endpoints, EndpointInfo{
			URL:         apiProxyEndpoint,
			Type:        "APIProxy",
			Access:      "cluster",
			Description: "Kubernetes API proxy (works anywhere kubectl works)",
		})
	}

	// Add cluster-internal endpoint if accessible (for pods/internal use)
	if clusterEndpoint := o.getClusterInternalEndpoint(svc); clusterEndpoint != "" {
		if o.canAccessClusterEndpoint(clusterEndpoint) {
			endpoints = append(endpoints, EndpointInfo{
				URL:         clusterEndpoint,
				Type:        "ClusterIP",
				Access:      "internal",
				Description: "Direct cluster-internal access (for pods)",
			})
		}
	}

	if len(endpoints) == 0 {
		return nil, fmt.Errorf("no accessible endpoints found for workspace %s", o.WorkspaceName)
	}

	return endpoints, nil
}

func (o *GetEndpointOptions) getLoadBalancerEndpoint(svc *corev1.Service) string {
	if svc.Spec.Type != "LoadBalancer" {
		return ""
	}

	for _, ingress := range svc.Status.LoadBalancer.Ingress {
		var endpoint string
		if ingress.IP != "" {
			endpoint = fmt.Sprintf("http://%s:80", ingress.IP)
		} else if ingress.Hostname != "" {
			endpoint = fmt.Sprintf("http://%s:80", ingress.Hostname)
		}
		if endpoint != "" {
			klog.V(3).Infof("Found external LoadBalancer endpoint: %s", endpoint)
			return endpoint
		}
	}
	return ""
}

func (o *GetEndpointOptions) getClusterInternalEndpoint(svc *corev1.Service) string {
	if svc.Spec.ClusterIP == "" || svc.Spec.ClusterIP == "None" {
		return ""
	}

	// Return cluster-internal endpoint (caller will check if accessible)
	clusterEndpoint := fmt.Sprintf("http://%s.%s.svc.cluster.local:80", o.WorkspaceName, o.Namespace)
	klog.V(3).Infof("Cluster-internal endpoint: %s", clusterEndpoint)
	return clusterEndpoint
}

// getAPIProxyEndpoint constructs the Kubernetes API proxy endpoint for the service
func (o *GetEndpointOptions) getAPIProxyEndpoint() (string, error) {
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

// canAccessClusterEndpoint checks if we can reach the cluster-internal endpoint
func (o *GetEndpointOptions) canAccessClusterEndpoint(endpoint string) bool {
	// Try to resolve the cluster DNS name
	_, err := net.LookupHost(strings.TrimPrefix(strings.TrimPrefix(endpoint, "http://"), "https://"))
	return err == nil
}
