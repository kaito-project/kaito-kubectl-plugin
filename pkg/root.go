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
	"fmt"

	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/klog/v2"
)

// NewRootCmd creates the root command for kubectl-kaito
func NewRootCmd(configFlags *genericclioptions.ConfigFlags, isPlugin bool) *cobra.Command {
	var cmdName = "kaito"
	if isPlugin {
		cmdName = "kubectl kaito"
	}

	cmd := &cobra.Command{
		Use:   cmdName,
		Short: "Kubernetes AI Toolchain Operator (Kaito) CLI",
		Long: `kubectl-kaito is a command-line tool for managing AI/ML model inference 
and fine-tuning workloads using the Kubernetes AI Toolchain Operator (Kaito).

This plugin simplifies the deployment, management, and monitoring of AI models
in Kubernetes clusters through Kaito workspaces.`,
		SilenceUsage: true,
		Example: fmt.Sprintf(`  # Deploy a model for inference
  %s deploy --workspace-name my-llama --model llama-2-7b --instance-type Standard_NC6s_v3

  # Check workspace status
  %s status --workspace-name my-llama

  # Get model inference endpoint
  %s get-endpoint --workspace-name my-llama

  # Interactive chat with deployed model
  %s chat --workspace-name my-llama

  # List supported models
  %s models list`, cmdName, cmdName, cmdName, cmdName, cmdName),
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			klog.V(4).Info("Initializing kubectl-kaito command")
			return nil
		},
	}

	// Add only essential global flags for Kaito users
	cmd.PersistentFlags().StringVar(configFlags.KubeConfig, "kubeconfig", *configFlags.KubeConfig, "Path to the kubeconfig file to use for CLI requests")
	cmd.PersistentFlags().StringVar(configFlags.Context, "context", *configFlags.Context, "The name of the kubeconfig context to use")
	cmd.PersistentFlags().StringVarP(configFlags.Namespace, "namespace", "n", *configFlags.Namespace, "If present, the namespace scope for this CLI request")

	return cmd
}
