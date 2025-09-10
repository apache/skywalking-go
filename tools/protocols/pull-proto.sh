#!/usr/bin/env bash
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
# Configuration
# -----------------------------
export GOPROXY=https://goproxy.cn,direct
PROTOC_VERSION=3.14.0
export PATH="/home/runner/go/bin:$PATH"
export GOPATH="${GOPATH:-$HOME/go}"
BASEDIR=$(dirname "$0")
TEMPDIR="$BASEDIR"/temp
BINDIR="$TEMPDIR"/bin
INCLUDE_DIR="$TEMPDIR"/include

SUBMODULE_PATH="skywalking-data-collect-protocol"
OUTPUT_BASE_DIR="../../protocols"

mkdir -p "$OUTPUT_BASE_DIR"
mkdir -p "$BINDIR"
mkdir -p "$INCLUDE_DIR"

# -----------------------------
# Install protoc (non-root)
# -----------------------------
if [[ ! -f "$BINDIR"/protoc ]]; then
    echo "Installing protoc locally..."
    UNAME=$(uname -s)
    if [[ "$UNAME" == "Linux" ]]; then
        PROTOC_ZIP="protoc-${PROTOC_VERSION}-linux-x86_64.zip"
    elif [[ "$UNAME" == "Darwin" ]]; then
        PROTOC_ZIP="protoc-${PROTOC_VERSION}-osx-x86_64.zip"
    elif [[ "$UNAME" == MINGW* ]] || [[ "$UNAME" == CYGWIN* ]]; then
        PROTOC_ZIP="protoc-${PROTOC_VERSION}-win64.zip"
    else
        echo "Unsupported OS: $UNAME"
        exit 1
    fi

    curl -sL "https://github.com/protocolbuffers/protobuf/releases/download/v${PROTOC_VERSION}/${PROTOC_ZIP}" -o "$TEMPDIR/$PROTOC_ZIP"
    
    # Extract to temp directory first
    EXTRACT_DIR="$TEMPDIR/extract"
    mkdir -p "$EXTRACT_DIR"
    unzip -o "$TEMPDIR/$PROTOC_ZIP" -d "$EXTRACT_DIR"
    
    # Copy protoc binary
    if [[ -f "$EXTRACT_DIR/bin/protoc" ]]; then
        cp "$EXTRACT_DIR/bin/protoc" "$BINDIR/protoc"
    elif [[ -f "$EXTRACT_DIR/bin/protoc.exe" ]]; then
        cp "$EXTRACT_DIR/bin/protoc.exe" "$BINDIR/protoc"
    else
        echo "Error: protoc binary not found in archive"
        exit 1
    fi
    
    # Copy include files
    if [[ -d "$EXTRACT_DIR/include" ]]; then
        cp -r "$EXTRACT_DIR/include"/* "$INCLUDE_DIR/"
    fi
    
    chmod +x "$BINDIR/protoc"
    rm -rf "$EXTRACT_DIR"
    rm -f "$TEMPDIR/$PROTOC_ZIP"
fi

# -----------------------------
# Export PATH for local protoc
# -----------------------------
export PATH="$BINDIR:$GOPATH/bin:$PATH"
rm -f "$GOPATH/bin/protoc-gen-go" "$GOPATH/bin/protoc-gen-go-grpc"
# -----------------------------
# Install Go plugins (fixed versions)
# -----------------------------
echo "Current go version:"
go version
if ! command -v protoc-gen-go &>/dev/null; then
    echo "Installing protoc-gen-go v1.26..."
    GO111MODULE=on GOPROXY=https://goproxy.cn,direct GOSUMDB=sum.golang.google.cn go install google.golang.org/protobuf/cmd/protoc-gen-go@v1.26.0
fi

if ! command -v protoc-gen-go-grpc &>/dev/null; then
    echo "Installing protoc-gen-go-grpc v1.1..."
    GO111MODULE=on GOPROXY=https://goproxy.cn,direct GOSUMDB=sum.golang.google.cn go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@v1.1.0

fi

# -----------------------------
# Find proto files
# -----------------------------
PROTO_FILES=$(find "$SUBMODULE_PATH" -name "*.proto")

# -----------------------------
# Generate Go gRPC code
# -----------------------------
echo "Starting gRPC code generation..."
for proto in $PROTO_FILES; do
    echo "Processing: $proto"
    "$BINDIR"/protoc --go_out="$OUTPUT_BASE_DIR" \
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
     -exec sed -i 's|"skywalking\.apache\.org/repo/goapi/|"github.com/apache/skywalking-go/protocols/|g' {} \;


echo "Reorganizing directory structure..."
if [ -d "$OUTPUT_BASE_DIR/skywalking.apache.org" ]; then

    if [ -d "$OUTPUT_BASE_DIR/skywalking.apache.org/repo/goapi/collect" ]; then
        mv "$OUTPUT_BASE_DIR/skywalking.apache.org/repo/goapi/collect" "$OUTPUT_BASE_DIR/"
    fi

    rm -rf "$OUTPUT_BASE_DIR/skywalking.apache.org" 2>/dev/null || true
fi

echo "Code generation completed. Output directory: $OUTPUT_BASE_DIR"
