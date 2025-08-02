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
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/dynamic"
	"k8s.io/klog/v2"
)

// StatusOptions holds the options for the status command
type StatusOptions struct {
	configFlags *genericclioptions.ConfigFlags

	WorkspaceName string
	Namespace     string
	Watch         bool
}

// NewStatusCmd creates the status command
func NewStatusCmd(configFlags *genericclioptions.ConfigFlags) *cobra.Command {
	o := &StatusOptions{
		configFlags: configFlags,
	}

	cmd := &cobra.Command{
		Use:   "status",
		Short: "Check status of Kaito workspaces",
		Long: `Check the status of one or more Kaito workspaces.

This command displays the current state of workspace resources, including
readiness conditions, resource allocation, and deployment status.`,
		Example: `  # Check status of a specific workspace
  kubectl kaito status --workspace-name my-workspace

  # Watch for changes in real-time
  kubectl kaito status --workspace-name my-workspace -n <namespace> --watch

  # Show detailed conditions and worker node information
  kubectl kaito status --workspace-name my-workspace --show-conditions --show-worker-nodes`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := o.validate(); err != nil {
				klog.Errorf("Validation failed: %v", err)
				return fmt.Errorf("validation failed: %w", err)
			}
			return o.Run()
		},
	}

	cmd.Flags().StringVar(&o.WorkspaceName, "workspace-name", "", "Name of the workspace to check")
	cmd.Flags().StringVarP(&o.Namespace, "namespace", "n", "", "Kubernetes namespace")
	cmd.Flags().BoolVarP(&o.Watch, "watch", "w", false, "Watch for changes in real-time")

	return cmd
}

func (o *StatusOptions) Run() error {
	klog.V(2).Info("Starting status command")

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

	// Get namespace
	if o.Namespace == "" {
		if ns, _, err := o.configFlags.ToRawKubeConfigLoader().Namespace(); err == nil && ns != "" {
			o.Namespace = ns
		} else {
			klog.V(4).Info("No namespace specified, using 'default'")
			o.Namespace = "default"
		}
	}

	// Handle watch mode for specific workspace
	if o.Watch {
		return o.watchWorkspace(dynamicClient)
	}

	return o.showWorkspaceStatus(dynamicClient)
}

// validates the status options
func (o *StatusOptions) validate() error {
	klog.V(4).Info("Validating status options")

	if o.WorkspaceName == "" {
		return fmt.Errorf("workspace name is required")
	}
	return nil
}

func (o *StatusOptions) showWorkspaceStatus(dynamicClient dynamic.Interface) error {
	klog.V(3).Infof("Getting status for workspace: %s", o.WorkspaceName)

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

	o.printWorkspaceDetails(workspace)

	return nil
}

func (o *StatusOptions) watchWorkspace(dynamicClient dynamic.Interface) error {
	klog.V(2).Infof("Starting watch for workspace: %s", o.WorkspaceName)
	fmt.Printf("Watching workspace %s for changes (Ctrl+C to stop)...\n", o.WorkspaceName)
	fmt.Println()

	gvr := schema.GroupVersionResource{
		Group:    "kaito.sh",
		Version:  "v1beta1",
		Resource: "workspaces",
	}

	watcher, err := dynamicClient.Resource(gvr).Namespace(o.Namespace).Watch(context.TODO(), metav1.ListOptions{
		FieldSelector: fmt.Sprintf("metadata.name=%s", o.WorkspaceName),
	})
	if err != nil {
		klog.Errorf("Failed to watch workspace: %v", err)
		return fmt.Errorf("failed to watch workspace: %w", err)
	}
	defer watcher.Stop()

	for event := range watcher.ResultChan() {
		if workspace, ok := event.Object.(*unstructured.Unstructured); ok {
			fmt.Printf("=== %s at %s ===\n", strings.ToUpper(string(event.Type)), time.Now().Format(time.RFC3339))
			o.printWorkspaceDetails(workspace)
			fmt.Println()
		}
	}

	return nil
}

func (o *StatusOptions) printWorkspaceDetails(workspace *unstructured.Unstructured) {
	klog.V(4).Info("Printing workspace details")

	fmt.Println("Workspace Details")
	fmt.Println("=================")
	fmt.Printf("Name: %s\n", workspace.GetName())
	fmt.Printf("Namespace: %s\n", workspace.GetNamespace())

	o.printResourceDetails(workspace)
	o.printWorkspaceMode(workspace)
	o.printDeploymentStatus(workspace)

	fmt.Printf("Age: %s\n", o.getAge(workspace))
	fmt.Println()
}

func (o *StatusOptions) printResourceDetails(workspace *unstructured.Unstructured) {
	// Get instance type and count from the top-level resource section (not spec.resource)
	if resource, found := workspace.Object["resource"]; found {
		if resourceMap, ok := resource.(map[string]interface{}); ok {
			o.printInstanceDetails(resourceMap)
			o.printPreferredNodes(resourceMap)
			o.printNodeSelector(resourceMap)
		}
	}
}

func (o *StatusOptions) printInstanceDetails(resourceMap map[string]interface{}) {
	if instanceType, found := resourceMap["instanceType"]; found {
		fmt.Printf("Instance Type: %s\n", instanceType)
	}
	if count, found := resourceMap["count"]; found {
		fmt.Printf("Node Count: %v\n", count)
	}
}

func (o *StatusOptions) printPreferredNodes(resourceMap map[string]interface{}) {
	// Display preferred nodes if available
	if preferredNodes, found := resourceMap["preferredNodes"]; found {
		if nodeList, ok := preferredNodes.([]interface{}); ok && len(nodeList) > 0 {
			fmt.Print("Preferred Nodes: ")
			for i, node := range nodeList {
				if i > 0 {
					fmt.Print(", ")
				}
				fmt.Print(node)
			}
			fmt.Println()
		}
	}
}

func (o *StatusOptions) printNodeSelector(resourceMap map[string]interface{}) {
	// Display node selector if available (alternative way to specify preferred nodes)
	if labelSelector, found := resourceMap["labelSelector"]; found {
		if labelMap, ok := labelSelector.(map[string]interface{}); ok {
			if matchLabels, found := labelMap["matchLabels"]; found {
				if labels, ok := matchLabels.(map[string]interface{}); ok && len(labels) > 0 {
					fmt.Print("Node Selector: ")
					first := true
					for key, value := range labels {
						if !first {
							fmt.Print(", ")
						}
						fmt.Printf("%s=%v", key, value)
						first = false
					}
					fmt.Println()
				}
			}
		}
	}
}

func (o *StatusOptions) printWorkspaceMode(workspace *unstructured.Unstructured) {
	// Check if tuning or inference (top-level, not spec.tuning)
	if _, found := workspace.Object["tuning"]; found {
		fmt.Println("Mode: Fine-tuning")
	} else {
		fmt.Println("Mode: Inference")
	}
}

func (o *StatusOptions) printDeploymentStatus(workspace *unstructured.Unstructured) {
	fmt.Println()
	fmt.Println("Deployment Status:")
	fmt.Println("==================")

	statusMap := o.getStatusMap(workspace)
	if statusMap == nil {
		return
	}

	o.printConditionStatuses(statusMap)
	o.printWorkerNodesList(statusMap)

	// Print detailed conditions
	fmt.Println()
	o.printConditions(workspace)
}

func (o *StatusOptions) getStatusMap(workspace *unstructured.Unstructured) map[string]interface{} {
	status, found := workspace.Object["status"]
	if !found {
		fmt.Println("Status: Not Available")
		return nil
	}

	statusMap, ok := status.(map[string]interface{})
	if !ok {
		fmt.Println("Status: Invalid Format")
		return nil
	}

	return statusMap
}

func (o *StatusOptions) printConditionStatuses(statusMap map[string]interface{}) {
	conditions, found := statusMap["conditions"]
	if !found {
		return
	}

	condList, ok := conditions.([]interface{})
	if !ok {
		return
	}

	resourceReady, inferenceReady, workspaceReady := o.extractConditionStatuses(condList)

	fmt.Printf("Resource Ready: %s\n", resourceReady)
	fmt.Printf("Inference Ready: %s\n", inferenceReady)
	fmt.Printf("Workspace Ready: %s\n", workspaceReady)
}

func (o *StatusOptions) extractConditionStatuses(condList []interface{}) (string, string, string) {
	resourceReady := "Unknown"
	inferenceReady := "Unknown"
	workspaceReady := "Unknown"

	for _, condition := range condList {
		if condMap, ok := condition.(map[string]interface{}); ok {
			condType, _ := condMap["type"].(string)
			condStatus, _ := condMap["status"].(string)

			switch condType {
			case "ResourceReady":
				resourceReady = condStatus
			case "InferenceReady":
				inferenceReady = condStatus
			case "WorkspaceSucceeded":
				workspaceReady = condStatus
			}
		}
	}

	return resourceReady, inferenceReady, workspaceReady
}

func (o *StatusOptions) printWorkerNodesList(statusMap map[string]interface{}) {
	workerNodes, found := statusMap["workerNodes"]
	if !found {
		return
	}

	nodeList, ok := workerNodes.([]interface{})
	if !ok || len(nodeList) == 0 {
		return
	}

	fmt.Println()
	fmt.Println("Worker Nodes:")
	for _, node := range nodeList {
		fmt.Printf("  %v\n", node)
	}
}

func (o *StatusOptions) printConditions(workspace *unstructured.Unstructured) {
	klog.V(4).Info("Printing workspace conditions")

	conditions, found, err := unstructured.NestedSlice(workspace.Object, "status", "conditions")
	if err != nil {
		klog.Errorf("Error getting conditions: %v", err)
		return
	}
	if !found || len(conditions) == 0 {
		fmt.Println("Detailed Conditions: None")
		return
	}

	fmt.Println("Detailed Conditions:")

	w := tabwriter.NewWriter(os.Stdout, 0, 8, 2, ' ', 0)
	defer w.Flush()
	fmt.Fprintln(w, "  STATUS\tMESSAGE\tLAST TRANSITION")

	for _, condition := range conditions {
		if condMap, ok := condition.(map[string]interface{}); ok {
			status, _ := condMap["status"].(string)
			message, _ := condMap["message"].(string)
			lastTransitionTime, _ := condMap["lastTransitionTime"].(string)

			fmt.Fprintf(w, "  %s\t%s\t%s\n",
				status, message, lastTransitionTime)
		}
	}

	fmt.Println()
}

func (o *StatusOptions) getAge(workspace *unstructured.Unstructured) string {
	creationTimestamp := workspace.GetCreationTimestamp()
	if creationTimestamp.IsZero() {
		klog.V(6).Infof("Creation timestamp not found for workspace %s", workspace.GetName())
		return "Unknown"
	}

	duration := time.Since(creationTimestamp.Time)

	switch {
	case duration.Seconds() < 60:
		return fmt.Sprintf("%ds", int(duration.Seconds()))
	case duration.Minutes() < 60:
		return fmt.Sprintf("%dm", int(duration.Minutes()))
	case duration.Hours() < 24:
		return fmt.Sprintf("%dh", int(duration.Hours()))
	default:
		return fmt.Sprintf("%dd", int(duration.Hours()/24))
	}
}
