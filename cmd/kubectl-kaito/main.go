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

	cmd "github.com/kaito-project/kaito-kubectl-plugin/pkg"
	"k8s.io/cli-runtime/pkg/genericclioptions"

	// Import auth plugins
	_ "k8s.io/client-go/plugin/pkg/client/auth"
)

func main() {
	// Determine if running as kubectl plugin
	isPlugin := strings.HasPrefix(filepath.Base(os.Args[0]), "kubectl-")

	// Create ConfigFlags to handle standard kubectl options
	configFlags := genericclioptions.NewConfigFlags(true)

	// Create and execute root command
	rootCmd := cmd.NewRootCmd(configFlags, isPlugin)
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
