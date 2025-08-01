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
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/cli-runtime/pkg/genericclioptions"
)

func TestStatusCmd(t *testing.T) {
	configFlags := genericclioptions.NewConfigFlags(true)
	cmd := NewStatusCmd(configFlags)

	t.Run("Command structure", func(t *testing.T) {
		assert.Equal(t, "status", cmd.Use)
		assert.Contains(t, cmd.Short, "status")
		assert.NotEmpty(t, cmd.Long)
		assert.NotEmpty(t, cmd.Example)
		assert.NotNil(t, cmd.RunE)
	})

	t.Run("Flags present", func(t *testing.T) {
		flags := cmd.Flags()

		workspaceFlag := flags.Lookup("workspace-name")
		assert.NotNil(t, workspaceFlag)

		allNamespacesFlag := flags.Lookup("all-namespaces")
		assert.NotNil(t, allNamespacesFlag)

		watchFlag := flags.Lookup("watch")
		assert.NotNil(t, watchFlag)
	})
}

func TestGetInstanceType(t *testing.T) {
	tests := []struct {
		name      string
		workspace *unstructured.Unstructured
		expected  string
	}{
		{
			name: "With instance type",
			workspace: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"spec": map[string]interface{}{
						"resource": map[string]interface{}{
							"instanceType": "Standard_NC6s_v3",
						},
					},
				},
			},
			expected: "Standard_NC6s_v3",
		},
		{
			name: "Missing instance type",
			workspace: &unstructured.Unstructured{
				Object: map[string]interface{}{},
			},
			expected: "Unknown",
		},
	}

	options := &StatusOptions{}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := options.getInstanceType(tt.workspace)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGetConditionStatus(t *testing.T) {
	tests := []struct {
		name          string
		workspace     *unstructured.Unstructured
		conditionType string
		expected      string
	}{
		{
			name: "Condition found",
			workspace: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"status": map[string]interface{}{
						"conditions": []interface{}{
							map[string]interface{}{
								"type":   "ResourceReady",
								"status": "True",
							},
						},
					},
				},
			},
			conditionType: "ResourceReady",
			expected:      "True",
		},
		{
			name: "Condition not found",
			workspace: &unstructured.Unstructured{
				Object: map[string]interface{}{},
			},
			conditionType: "ResourceReady",
			expected:      "Unknown",
		},
	}

	options := &StatusOptions{}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := options.getConditionStatus(tt.workspace, tt.conditionType)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGetAge(t *testing.T) {
	now := time.Now()
	oneHourAgo := now.Add(-time.Hour)

	workspace := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"metadata": map[string]interface{}{
				"creationTimestamp": oneHourAgo.Format(time.RFC3339),
			},
		},
	}

	options := &StatusOptions{}
	result := options.getAge(workspace)

	// Should contain some time duration
	assert.NotEqual(t, "<unknown>", result)
	assert.NotEmpty(t, result)
}
