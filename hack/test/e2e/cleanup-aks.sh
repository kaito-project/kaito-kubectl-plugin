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

# cleanup-aks.sh - Cleanup AKS cluster after e2e tests

set -e

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CLUSTER_INFO_FILE="${SCRIPT_DIR}/aks-cluster-info.env"

echo -e "${BLUE}🧹 Cleaning up AKS cluster and resources${NC}"

# Check if Azure CLI is available
if ! command -v az >/dev/null 2>&1; then
    echo -e "${YELLOW}⚠️  Azure CLI not found, cannot clean up AKS resources${NC}"
    exit 1
fi

# Check if authenticated
if ! az account show >/dev/null 2>&1; then
    echo -e "${RED}❌ Not authenticated with Azure. Please run 'az login' first.${NC}"
    exit 1
fi

# Load cluster info if available
if [ -f "${CLUSTER_INFO_FILE}" ]; then
    echo -e "${BLUE}📋 Loading cluster info from ${CLUSTER_INFO_FILE}${NC}"
    source "${CLUSTER_INFO_FILE}"
else
    echo -e "${YELLOW}⚠️  Cluster info file not found. Using environment variables or defaults.${NC}"
fi

# Use environment variables or prompt for values
if [ -z "$AKS_RESOURCE_GROUP" ]; then
    echo -e "${YELLOW}🔍 AKS_RESOURCE_GROUP not set. Please provide the resource group name:${NC}"
    read -p "Resource Group: " AKS_RESOURCE_GROUP
fi

if [ -z "$AKS_CLUSTER_NAME" ]; then
    echo -e "${YELLOW}🔍 AKS_CLUSTER_NAME not set. Please provide the cluster name:${NC}"
    read -p "Cluster Name: " AKS_CLUSTER_NAME
fi

if [ -z "$AKS_RESOURCE_GROUP" ] || [ -z "$AKS_CLUSTER_NAME" ]; then
    echo -e "${RED}❌ Missing required information. Cannot proceed with cleanup.${NC}"
    exit 1
fi

echo -e "${BLUE}📋 Cleanup configuration:${NC}"
echo -e "   Resource Group: ${AKS_RESOURCE_GROUP}"
echo -e "   Cluster Name: ${AKS_CLUSTER_NAME}"

# Get confirmation unless in CI
if [ -z "$CI" ] && [ -z "$SKIP_CONFIRMATION" ]; then
    echo ""
    echo -e "${YELLOW}⚠️  This will delete the entire resource group and all resources within it!${NC}"
    echo -n "Are you sure you want to proceed? (y/N): "
    read -r response
    if [[ ! "$response" =~ ^[Yy]$ ]]; then
        echo "Cancelled."
        exit 0
    fi
fi

# Check if resource group exists
if ! az group show --name "${AKS_RESOURCE_GROUP}" >/dev/null 2>&1; then
    echo -e "${GREEN}✅ Resource group ${AKS_RESOURCE_GROUP} does not exist, nothing to clean up${NC}"
    rm -f "${CLUSTER_INFO_FILE}"
    exit 0
fi

# Delete the resource group (this deletes the cluster and all associated resources)
echo -e "${YELLOW}🗑️  Deleting resource group: ${AKS_RESOURCE_GROUP}${NC}"
echo -e "${BLUE}   This will delete the AKS cluster and all associated resources...${NC}"

az group delete --name "${AKS_RESOURCE_GROUP}" --yes --no-wait

echo -e "${GREEN}✅ AKS cluster deletion initiated${NC}"
echo -e "${BLUE}💡 Deletion is running in the background and may take several minutes to complete.${NC}"
echo -e "${BLUE}   You can check the status in the Azure portal or with:${NC}"
echo -e "${BLUE}   az group show --name ${AKS_RESOURCE_GROUP}${NC}"

# Clean up cluster info file
if [ -f "${CLUSTER_INFO_FILE}" ]; then
    rm -f "${CLUSTER_INFO_FILE}"
    echo -e "${BLUE}🗑️  Removed cluster info file${NC}"
fi

echo -e "${GREEN}✅ AKS cluster cleanup complete!${NC}" 