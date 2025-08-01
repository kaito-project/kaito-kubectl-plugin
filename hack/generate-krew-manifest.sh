#!/bin/bash

# Script to generate complete krew manifest with real SHA256 values
# Usage: ./hack/generate-krew-manifest-complete.sh v1.0.0

if [ -z "$1" ]; then
    echo "Usage: $0 <tag>"
    echo "Example: $0 v1.0.0"
    exit 1
fi

TAG=$1
VERSION=${TAG#v}  # Remove 'v' prefix for filenames
REPO="kaito-project/kaito-kubectl-plugin"
TEMPLATE_FILE="krew/kaito.yaml"
OUTPUT_FILE="krew/kaito-${TAG}.yaml"

echo "Generating complete krew manifest for tag: $TAG"

# Function to get SHA256 of a URL
get_sha256() {
    local url=$1
    echo "Getting SHA256 for $url..." >&2  # Send to stderr, not stdout
    curl -sL "$url" | sha256sum | cut -d' ' -f1
}

# Function to process addURIAndSha template function
process_addURIAndSha() {
    local template_line="$1"
    local url=$(echo "$template_line" | sed -n 's/.*{{addURIAndSha "\([^"]*\)".*/\1/p')
    local sha256=$(get_sha256 "$url")
    
    echo "    uri: $url"
    echo "    sha256: $sha256"
}

# Check if template exists
if [ ! -f "$TEMPLATE_FILE" ]; then
    echo "Error: Template file $TEMPLATE_FILE not found"
    exit 1
fi

# Process the template
{
    while IFS= read -r line; do
        # Replace basic template variables
        line=$(echo "$line" | sed -e "s/{{ \.TagName }}/${TAG}/g" -e "s/{{ \.Version }}/${VERSION}/g")
        
        # Check if line contains addURIAndSha function
        if [[ "$line" == *"{{addURIAndSha"* ]]; then
            # Process the addURIAndSha function
            process_addURIAndSha "$line"
        else
            # Output line as-is
            echo "$line"
        fi
    done < "$TEMPLATE_FILE"
} > "$OUTPUT_FILE"

echo "Generated complete krew manifest: $OUTPUT_FILE"
echo "You can now test it with: kubectl krew install --manifest=$OUTPUT_FILE"