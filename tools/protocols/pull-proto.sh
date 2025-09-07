#!/bin/bash
# Licensed to the Apache Software Foundation (ASF) under one
# or more contributor license agreements.  See the NOTICE file
# distributed with this work for additional information
# regarding copyright ownership.  The ASF licenses this file
# to you under the Apache License, Version 2.0 (the
# "License"); you may not use this file except in compliance
# with the License.  You may obtain a copy of the License at
#
#   http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing,
# software distributed under the License is distributed on an
# "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
# KIND, either express or implied.  See the License for the
# specific language governing permissions and limitations
# under the License.

set -e

# -----------------------------
# Configuration parameters
# -----------------------------
SUBMODULE_PATH="skywalking-data-collect-protocol"
OUTPUT_BASE_DIR="../../protocols"

mkdir -p "$OUTPUT_BASE_DIR"

# -----------------------------
# Find proto files
# -----------------------------
PROTO_FILES=$(find "$SUBMODULE_PATH" -name "*.proto")

# -----------------------------
# Check protoc and plugins
# -----------------------------
if ! command -v protoc &> /dev/null; then
  echo "Error: protoc is not installed. Please install the Protocol Buffers compiler first."
  exit 1
fi

check_plugin() {
  if ! command -v $1 &> /dev/null; then
    echo "Error: $2 plugin is not installed. Please install it first."
    exit 1
  fi
}

check_plugin "protoc-gen-go" "Go gRPC plugin"
check_plugin "protoc-gen-go-grpc" "Go gRPC service plugin"

# -----------------------------
# Generate Go gRPC code
# -----------------------------
echo "Starting gRPC code generation..."
for proto in $PROTO_FILES; do
  echo "Processing file: $proto"
  protoc --go_out="$OUTPUT_BASE_DIR" \
         --go_opt=paths=import \
         --go-grpc_out="$OUTPUT_BASE_DIR" \
         --go-grpc_opt=paths=import \
         -I "$SUBMODULE_PATH" \
         "$proto"
done

# -----------------------------
# Fix import paths
# -----------------------------
echo "Modifying import paths in generated Go files..."
find "$OUTPUT_BASE_DIR" -name "*.pb.go" \
     -exec sed -i 's|"skywalking\.apache\.org/|"github.com/apache/skywalking-go/protocols/skywalking.apache.org/|g' {} \;

echo "Code generation completed. Output directory: $OUTPUT_BASE_DIR"