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

	"github.com/stretchr/testify/assert"
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

		watchFlag := flags.Lookup("watch")
		assert.NotNil(t, watchFlag)
	})
}

