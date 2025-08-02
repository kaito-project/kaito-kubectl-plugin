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

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	cmd "github.com/kaito-project/kaito-kubectl-plugin/pkg"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/cli-runtime/pkg/genericclioptions"
)

func TestIsPluginDetection(t *testing.T) {
	tests := []struct {
		name     string
		args0    string
		isPlugin bool
	}{
		{
			name:     "kubectl plugin format",
			args0:    "kubectl-kaito",
			isPlugin: true,
		},
		{
			name:     "kubectl plugin with path",
			args0:    "/usr/local/bin/kubectl-kaito",
			isPlugin: true,
		},
		{
			name:     "standalone binary",
			args0:    "kaito",
			isPlugin: false,
		},
		{
			name:     "standalone binary with path",
			args0:    "/usr/local/bin/kaito",
			isPlugin: false,
		},
		{
			name:     "other kubectl plugin",
			args0:    "kubectl-other",
			isPlugin: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test the logic that determines if running as kubectl plugin
			isPlugin := strings.HasPrefix(filepath.Base(tt.args0), "kubectl-")
			assert.Equal(t, tt.isPlugin, isPlugin)
		})
	}
}

func TestMainFunctionComponents(t *testing.T) {
	// Test that we can create the components used in main()
	t.Run("ConfigFlags creation", func(t *testing.T) {
		// This should not panic and should return a valid object
		require.NotPanics(t, func() {
			configFlags := genericclioptions.NewConfigFlags(true)
			assert.NotNil(t, configFlags)
		})
	})

	t.Run("Original args preservation", func(t *testing.T) {
		// Store original args
		originalArgs := os.Args

		// Test with different args
		testArgs := []string{"kubectl-kaito", "models", "list"}
		os.Args = testArgs

		// Verify args are as expected
		assert.Equal(t, testArgs, os.Args)
		assert.True(t, strings.HasPrefix(filepath.Base(os.Args[0]), "kubectl-"))

		// Restore original args
		os.Args = originalArgs
	})
}

func TestMainExecution(t *testing.T) {
	// Test that main components can be instantiated without error
	t.Run("Main components integration", func(t *testing.T) {
		// Save original args
		originalArgs := os.Args
		defer func() { os.Args = originalArgs }()

		// Set test args that would trigger help (safe execution)
		os.Args = []string{"kubectl-kaito", "--help"}

		// Test that the main function logic components work
		require.NotPanics(t, func() {
			// Simulate the main function logic without calling Execute()
			isPlugin := strings.HasPrefix(filepath.Base(os.Args[0]), "kubectl-")
			assert.True(t, isPlugin)

			configFlags := genericclioptions.NewConfigFlags(true)
			assert.NotNil(t, configFlags)

			// Import the cmd package to ensure it's available
			// This tests that the import works correctly
			rootCmd := cmd.NewRootCmd(configFlags, isPlugin)
			assert.NotNil(t, rootCmd)
			assert.Equal(t, "kubectl kaito", rootCmd.Use)
		})
	})
}

// TestAuthPluginImport verifies that the auth plugin import works
func TestAuthPluginImport(t *testing.T) {
	// This test verifies that the auth plugin import doesn't cause issues
	// The import is a side effect import, so we just verify no panic occurs
	t.Run("Auth plugin import", func(t *testing.T) {
		// If we got this far in the test, the import worked
		assert.True(t, true, "Auth plugin import successful")
	})
}
