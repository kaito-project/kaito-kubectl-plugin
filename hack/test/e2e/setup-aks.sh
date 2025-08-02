#!/bin/bash

# Copyright (c) 2024 Kaito Project
# 
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
# 
#     http://www.apache.org/licenses/LICENSE-2.0
# 
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

# setup-aks.sh - Setup AKS cluster and prerequisites for e2e tests

set -e

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"

# Default values - can be overridden by environment variables
AKS_CLUSTER_NAME="${AKS_CLUSTER_NAME:-kaito-e2e-aks-$(date +%s)}"
AKS_RESOURCE_GROUP="${AKS_RESOURCE_GROUP:-kaito-e2e-rg-$(date +%s)}"
AKS_LOCATION="${AKS_LOCATION:-eastus}"
AKS_NODE_VM_SIZE="${AKS_NODE_VM_SIZE:-Standard_NC6s_v3}"
AKS_NODE_COUNT="${AKS_NODE_COUNT:-1}"
KUBERNETES_VERSION="${KUBERNETES_VERSION:-1.33.0}"

TIMEOUT_CLUSTER=20m
TIMEOUT_INSTALL=5m

echo -e "${BLUE}🚀 Setting up AKS cluster and prerequisites for e2e tests${NC}"
echo -e "${YELLOW}⚠️  Warning: This will create billable Azure resources!${NC}"
echo -e "${YELLOW}   Estimated cost: $2-5 for a 1-hour test run${NC}"

# Function to check if command exists
command_exists() {
    command -v "$1" >/dev/null 2>&1
}

# Function to install kubectl
install_kubectl() {
    echo -e "${YELLOW}📦 Installing kubectl...${NC}"
    
    if command_exists brew; then
        brew install kubectl
    elif command_exists curl; then
        curl -LO "https://dl.k8s.io/release/$(curl -L -s https://dl.k8s.io/release/stable.txt)/bin/linux/amd64/kubectl"
        chmod +x kubectl
        sudo mv kubectl /usr/local/bin/
    else
        echo -e "${RED}❌ Unable to install kubectl: no suitable package manager found${NC}"
        exit 1
    fi
}

# Function to install Azure CLI
install_azure_cli() {
    echo -e "${YELLOW}📦 Installing Azure CLI...${NC}"
    
    if command_exists brew; then
        brew install azure-cli
    elif command_exists apt-get; then
        curl -sL https://aka.ms/InstallAzureCLIDeb | sudo bash
    elif command_exists yum; then
        sudo rpm --import https://packages.microsoft.com/keys/microsoft.asc
        echo -e "[azure-cli]\nname=Azure CLI\nbaseurl=https://packages.microsoft.com/yumrepos/azure-cli\nenabled=1\ngpgcheck=1\ngpgkey=https://packages.microsoft.com/keys/microsoft.asc" | sudo tee /etc/yum.repos.d/azure-cli.repo
        sudo yum install azure-cli
    elif command_exists curl; then
        curl -L https://aka.ms/InstallAzureCli | bash
    else
        echo -e "${RED}❌ Unable to install Azure CLI: no suitable package manager found${NC}"
        exit 1
    fi
}

# Check and install prerequisites
echo -e "${BLUE}🔍 Checking prerequisites...${NC}"

# Check kubectl
if ! command_exists kubectl; then
    install_kubectl
else
    echo -e "${GREEN}✅ kubectl is available${NC}"
fi

# Check helm
if ! command_exists helm; then
    install_helm
else
    echo -e "${GREEN}✅ helm is available${NC}"
fi

# Check Azure CLI
if ! command_exists az; then
    install_azure_cli
else
    echo -e "${GREEN}✅ Azure CLI is available${NC}"
fi

# Check Azure authentication
echo -e "${BLUE}🔐 Checking Azure authentication...${NC}"
if ! az account show >/dev/null 2>&1; then
    echo -e "${YELLOW}⚠️  Not logged into Azure${NC}"
    echo -e "${BLUE}🔑 Please log into Azure:${NC}"
    az login
    
    if ! az account show >/dev/null 2>&1; then
        echo -e "${RED}❌ Azure authentication failed${NC}"
        exit 1
    fi
fi

echo -e "${GREEN}✅ Azure CLI is authenticated${NC}"
az account show --output table

# Get confirmation unless in CI
if [ -z "$CI" ] && [ -z "$SKIP_CONFIRMATION" ]; then
    echo ""
    echo -e "${YELLOW}⚠️  About to create AKS cluster with the following configuration:${NC}"
    echo -e "   Cluster Name: ${AKS_CLUSTER_NAME}"
    echo -e "   Resource Group: ${AKS_RESOURCE_GROUP}"
    echo -e "   Location: ${AKS_LOCATION}"
    echo -e "   VM Size: ${AKS_NODE_VM_SIZE}"
    echo -e "   Node Count: ${AKS_NODE_COUNT}"
    echo ""
    echo -n "Do you want to proceed? (y/N): "
    read -r response
    if [[ ! "$response" =~ ^[Yy]$ ]]; then
        echo "Cancelled."
        exit 0
    fi
fi

# Clean up any existing resources
echo -e "${BLUE}🧹 Cleaning up any existing resources...${NC}"
if az group show --name "${AKS_RESOURCE_GROUP}" >/dev/null 2>&1; then
    echo -e "${YELLOW}⚠️  Deleting existing resource group: ${AKS_RESOURCE_GROUP}${NC}"
    az group delete --name "${AKS_RESOURCE_GROUP}" --yes --no-wait
    echo -e "${BLUE}⏳ Waiting for resource group deletion to complete...${NC}"
    while az group show --name "${AKS_RESOURCE_GROUP}" >/dev/null 2>&1; do
        sleep 30
        echo -e "${BLUE}   Still waiting for deletion...${NC}"
    done
fi

# Create resource group
echo -e "${BLUE}🏗️  Creating resource group: ${AKS_RESOURCE_GROUP}${NC}"
az group create --name "${AKS_RESOURCE_GROUP}" --location "${AKS_LOCATION}"

# Create AKS cluster
echo -e "${BLUE}🏗️  Creating AKS cluster: ${AKS_CLUSTER_NAME}${NC}"
echo -e "${YELLOW}   This may take 10-15 minutes...${NC}"

timeout "${TIMEOUT_CLUSTER}" az aks create \
    --resource-group "${AKS_RESOURCE_GROUP}" \
    --name "${AKS_CLUSTER_NAME}" \
    --node-count "${AKS_NODE_COUNT}" \
    --node-vm-size "${AKS_NODE_VM_SIZE}" \
    --generate-ssh-keys \
    --enable-oidc-issuer \
    --enable-ai-toolchain-operator \
    --kubernetes-version "${KUBERNETES_VERSION}"

# Get credentials
echo -e "${BLUE}🔑 Getting AKS credentials...${NC}"
az aks get-credentials \
    --resource-group "${AKS_RESOURCE_GROUP}" \
    --name "${AKS_CLUSTER_NAME}" \
    --overwrite-existing

echo -e "${BLUE}⏳ Waiting for cluster to be ready...${NC}"
timeout "${TIMEOUT_CLUSTER}" kubectl wait --for=condition=Ready nodes --all --timeout=900s

# Create kaito-system namespace
kubectl create namespace kaito-system --dry-run=client -o yaml | kubectl apply -f -

echo -e "${GREEN}✅ AKS cluster setup complete!${NC}"
echo -e "${GREEN}   Cluster: ${AKS_CLUSTER_NAME}${NC}"
echo -e "${GREEN}   Resource Group: ${AKS_RESOURCE_GROUP}${NC}"
echo -e "${GREEN}   Location: ${AKS_LOCATION}${NC}"
echo -e "${GREEN}   Context: ${AKS_CLUSTER_NAME}${NC}"

# Save cluster info for cleanup
cat > "${SCRIPT_DIR}/aks-cluster-info.env" <<EOF
AKS_CLUSTER_NAME="${AKS_CLUSTER_NAME}"
AKS_RESOURCE_GROUP="${AKS_RESOURCE_GROUP}"
AKS_LOCATION="${AKS_LOCATION}"
EOF

echo -e "${BLUE}💾 Cluster info saved to: ${SCRIPT_DIR}/aks-cluster-info.env${NC}"
echo -e "${BLUE}🎯 Ready to run AKS e2e tests!${NC}"
echo ""
echo -e "${YELLOW}💡 To clean up after tests, run: ./hack/test/e2e/cleanup-aks.sh${NC}" 