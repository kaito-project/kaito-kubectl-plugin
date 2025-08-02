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

package e2e

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

const (
	binaryName      = "kubectl-kaito"
	testTimeout     = 30 * time.Second
	buildTimeout    = 60 * time.Second
	longTestTimeout = 120 * time.Second
)

var (
	binaryPath string
)

func runCommand(t *testing.T, timeout time.Duration, args ...string) (string, string, error) {
	if timeout == 0 {
		timeout = testTimeout
	}

	// Check if binary exists
	if _, err := os.Stat(binaryPath); err != nil {
		return "", "", fmt.Errorf("binary not found at %s: %v", binaryPath, err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, binaryPath, args...)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	return strings.TrimSpace(stdout.String()), strings.TrimSpace(stderr.String()), err
}

func runKubectlCommand(t *testing.T, timeout time.Duration, args ...string) (string, error) {
	if timeout == 0 {
		timeout = testTimeout
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "kubectl", args...)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	return strings.TrimSpace(stdout.String()), err
}

// Common test functions used in the e2e tests

func verifyClusterReady(t *testing.T) {
	stdout, err := runKubectlCommand(t, testTimeout, "get", "nodes")
	if err != nil {
		t.Errorf("Failed to get nodes: %v\nStdout: %s", err, stdout)
		return
	}

	if !strings.Contains(stdout, "Ready") {
		t.Errorf("No Ready nodes found. Output: %s", stdout)
		return
	}

	t.Logf("✅ Cluster is ready with nodes: %s", stdout)
}

func verifyKaitoOperator(t *testing.T) {
	// Check for Kaito operator in kaito-system namespace first
	stdout, err := runKubectlCommand(t, longTestTimeout, "get", "deployment", "kaito-controller-manager", "-n", "kaito-system")
	if err != nil {
		// If not found in kaito-system, check in kube-system (AKS managed add-on)
		stdout, err = runKubectlCommand(t, longTestTimeout, "get", "deployment", "-n", "kube-system", "-l", "app=ai-toolchain-operator")
		if err != nil {
			// Try checking pods instead
			stdout, err = runKubectlCommand(t, longTestTimeout, "get", "pods", "-n", "kube-system", "-l", "app=kaito-workspace")
			if err != nil {
				t.Errorf("Failed to get Kaito operator: %s\nStderr: %s", err, stdout)
				return
			}
		}
	}

	if !strings.Contains(stdout, "kaito") {
		t.Errorf("Kaito operator not found. Output: %s", stdout)
		return
	}

	t.Logf("✅ Kaito operator is ready")
}

func testStatusNoWorkspaces(t *testing.T) {
	stdout, stderr, err := runCommand(t, testTimeout, "status")

	// Should succeed even with no workspaces
	combinedOutput := stdout + stderr

	if err != nil {
		// Check if it's a meaningful error (like CRD not found)
		if !strings.Contains(combinedOutput, "no resources found") &&
			!strings.Contains(combinedOutput, "the server doesn't have a resource type") {
			t.Errorf("Unexpected error: %v\nOutput: %s", err, combinedOutput)
			return
		}
	}

	t.Logf("✅ Status command handles no workspaces correctly")
}

func testGetEndpointNoWorkspace(t *testing.T) {
	_, _, err := runCommand(t, testTimeout, "get-endpoint", "--workspace-name", "nonexistent")

	if err == nil {
		t.Errorf("Expected error for nonexistent workspace, but command succeeded")
		return
	}

	t.Logf("✅ Get-endpoint correctly handles nonexistent workspace")
}

func testModelsList(t *testing.T) {
	stdout, stderr, err := runCommand(t, longTestTimeout, "models", "list")

	// Should succeed or gracefully handle network failures
	combinedOutput := stdout + stderr

	if err != nil {
		// If network fails, should show fallback models
		if !strings.Contains(combinedOutput, "phi-3.5-mini-instruct") &&
			!strings.Contains(combinedOutput, "llama-2-7b") {
			t.Errorf("Expected fallback models not found. Output: %s", combinedOutput)
			return
		}
	} else {
		// If succeeds, should show model list
		if !strings.Contains(combinedOutput, "NAME") || !strings.Contains(combinedOutput, "TYPE") {
			t.Errorf("Expected model list headers not found. Output: %s", combinedOutput)
			return
		}
	}

	t.Logf("✅ Models list successful")
}

func testModelsDescribe(t *testing.T) {
	stdout, stderr, err := runCommand(t, testTimeout, "models", "describe", "phi-3.5-mini-instruct")

	combinedOutput := stdout + stderr

	if err != nil {
		// Should provide helpful error message
		if !strings.Contains(combinedOutput, "phi-3.5-mini-instruct") {
			t.Errorf("Expected model name in error message. Output: %s", combinedOutput)
			return
		}
	} else {
		// If succeeds, should show model details
		if !strings.Contains(combinedOutput, "phi-3.5-mini-instruct") {
			t.Errorf("Expected model details not found. Output: %s", combinedOutput)
			return
		}
	}

	t.Logf("✅ Models describe successful")
}

func testHelpCommands(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{"root help", []string{"--help"}},
		{"models help", []string{"models", "--help"}},
		{"deploy help", []string{"deploy", "--help"}},
		{"status help", []string{"status", "--help"}},
		{"get-endpoint help", []string{"get-endpoint", "--help"}},
		{"chat help", []string{"chat", "--help"}},
		// TODO: Uncomment when RAG command is available
		// {"rag help", []string{"rag", "--help"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stdout, stderr, err := runCommand(t, testTimeout, tt.args...)

			// Help should exit with code 0
			if err != nil {
				t.Errorf("Help command failed: %v\nStdout: %s\nStderr: %s", err, stdout, stderr)
				return
			}

			// Should have some help content
			combinedOutput := stdout + stderr
			if !strings.Contains(combinedOutput, "Usage:") && !strings.Contains(combinedOutput, "kubectl kaito") {
				t.Errorf("Expected help content not found in output: %s", combinedOutput)
				return
			}

			t.Logf("✅ %s successful", tt.name)
		})
	}
}

func testInputValidation(t *testing.T) {
	tests := []struct {
		name        string
		args        []string
		expectError bool
	}{
		{
			name:        "deploy missing workspace name",
			args:        []string{"deploy", "--model", "llama-2-7b"},
			expectError: true,
		},
		{
			name:        "deploy missing model",
			args:        []string{"deploy", "--workspace-name", "test"},
			expectError: true,
		},
		{
			name:        "deploy invalid model",
			args:        []string{"deploy", "--workspace-name", "test", "--model", "invalid-model"},
			expectError: true,
		},
		{
			name:        "chat missing workspace",
			args:        []string{"chat", "--message", "hello"},
			expectError: true,
		},
		{
			name:        "get-endpoint missing workspace",
			args:        []string{"get-endpoint"},
			expectError: true,
		},
		{
			name:        "rag deploy missing name",
			args:        []string{"rag", "deploy", "--vector-db", "faiss"},
			expectError: true,
		},
		{
			name:        "valid dry-run should succeed",
			args:        []string{"deploy", "--workspace-name", "test", "--model", "phi-3.5-mini-instruct", "--dry-run"},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stdout, stderr, err := runCommand(t, testTimeout, tt.args...)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error but command succeeded. Stdout: %s, Stderr: %s", stdout, stderr)
					return
				}
				// For kubectl plugins with SilenceErrors=true, we just check that it exits with non-zero code
				// The actual error messages are suppressed, which is correct behavior
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v\nStdout: %s\nStderr: %s", err, stdout, stderr)
					return
				}
			}

			t.Logf("✅ %s validation successful", tt.name)
		})
	}
}
