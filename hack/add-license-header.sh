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

set -euo pipefail

# License header to add
LICENSE_HEADER='/*
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
*/'

# Function to check if file has license header
has_license_header() {
    local file="$1"
    if grep -q "Copyright.*Kaito Project" "$file" 2>/dev/null; then
        return 0
    fi
    return 1
}

# Function to add license header to a file
add_license_header() {
    local file="$1"
    local temp_file
    temp_file=$(mktemp)
    
    # Add license header followed by original content
    echo "$LICENSE_HEADER" > "$temp_file"
    echo "" >> "$temp_file"
    cat "$file" >> "$temp_file"
    
    # Replace original file
    mv "$temp_file" "$file"
    echo "Added license header to $file"
}

# Main logic
exit_code=0

for file in "$@"; do
    # Skip if not a Go file
    if [[ ! "$file" =~ \.go$ ]]; then
        continue
    fi
    
    # Skip if file doesn't exist
    if [[ ! -f "$file" ]]; then
        continue
    fi
    
    # Skip generated files
    if [[ "$file" =~ (\.pb\.go|\.gen\.go|zz_generated.*\.go)$ ]]; then
        continue
    fi
    
    # Skip vendor directory
    if [[ "$file" =~ vendor/ ]]; then
        continue
    fi
    
    # Check if file needs license header
    if ! has_license_header "$file"; then
        add_license_header "$file"
        exit_code=1  # Indicate files were modified
    fi
done

exit $exit_code 