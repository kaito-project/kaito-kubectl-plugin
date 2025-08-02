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
	"strings"
	"testing"
)

// TestAKSClusterOperations tests kubectl-kaito plugin functionality on AKS cluster
func TestAKSClusterOperations(t *testing.T) {
	t.Run("verify_cluster_ready", func(t *testing.T) {
		verifyClusterReady(t)
	})

	t.Run("verify_kaito_operator", func(t *testing.T) {
		verifyKaitoOperator(t)
	})

	t.Run("deploy_validation", func(t *testing.T) {
		testAKSDeployValidation(t)
	})

	t.Run("status_no_workspaces", func(t *testing.T) {
		testStatusNoWorkspaces(t)
	})

	t.Run("get_endpoint_no_workspace", func(t *testing.T) {
		testGetEndpointNoWorkspace(t)
	})

	t.Run("models_list", func(t *testing.T) {
		testModelsList(t)
	})

	t.Run("models_describe", func(t *testing.T) {
		testModelsDescribe(t)
	})

	t.Run("chat_validation", func(t *testing.T) {
		testChatValidation(t)
	})

	t.Run("help_commands", func(t *testing.T) {
		testHelpCommands(t)
	})

	t.Run("input_validation", func(t *testing.T) {
		testInputValidation(t)
	})
}

// testAKSDeployValidation tests deployment validation on AKS with GPU instance types
func testAKSDeployValidation(t *testing.T) {
	stdout, stderr, err := runCommand(t, testTimeout,
		"deploy",
		"--workspace-name", "test-gpu-workspace",
		"--model", "phi-2",
		"--instance-type", "Standard_NC6s_v3",
		"--dry-run")

	if err != nil {
		t.Errorf("Deploy validation failed: %v\nStdout: %s\nStderr: %s", err, stdout, stderr)
		return
	}

	combinedOutput := stdout + stderr
	if !strings.Contains(combinedOutput, "Dry-run mode") {
		t.Errorf("Expected dry-run output not found: %s", combinedOutput)
	}

	if !strings.Contains(combinedOutput, "Standard_NC6s_v3") {
		t.Errorf("Expected GPU instance type not found in output: %s", combinedOutput)
	}

	t.Logf("✅ AKS deploy validation successful")
}

// testChatValidation tests chat command validation (without actual chat)
func testChatValidation(t *testing.T) {
	// Test missing workspace
	_, _, err := runCommand(t, testTimeout, "chat", "--message", "hello")
	if err == nil {
		t.Errorf("Expected error for missing workspace, but command succeeded")
	}
	t.Logf("✅ Chat validation correctly requires workspace")

	// Test with workspace but no endpoint (should fail gracefully)
	_, _, err = runCommand(t, testTimeout,
		"chat",
		"--workspace-name", "nonexistent-workspace",
		"--message", "hello")
	if err == nil {
		t.Errorf("Expected error for nonexistent workspace, but command succeeded")
	}
	t.Logf("✅ Chat validation correctly handles nonexistent workspace")
}
