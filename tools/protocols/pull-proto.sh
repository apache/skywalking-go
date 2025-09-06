#!/bin/bash

# Configuration parameters
REPO_URL="https://github.com/apache/skywalking-data-collect-protocol.git"
LOCAL_REPO_DIR="skywalking-data-collect-protocol"
OUTPUT_BASE_DIR="../../protocols"

mkdir -p $OUTPUT_BASE_DIR

if [ -d "$LOCAL_REPO_DIR" ]; then
  echo "Updating repository..."
  cd $LOCAL_REPO_DIR
  git pull
  cd ..
else
  echo "Cloning repository..."
  git clone $REPO_URL
fi

PROTO_FILES=$(find $LOCAL_REPO_DIR -name "*.proto")

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

echo "Starting gRPC code generation..."

for proto in $PROTO_FILES; do
  echo "Processing file: $proto"

  protoc --go_out=$OUTPUT_BASE_DIR \
         --go_opt=paths=import \
         --go-grpc_out=$OUTPUT_BASE_DIR \
         --go-grpc_opt=paths=import \
         -I $LOCAL_REPO_DIR \
         $proto

done

echo "Modifying import paths in generated Go files..."
find $OUTPUT_BASE_DIR -name "*.pb.go" -exec sed -i 's|"skywalking\.apache\.org/|"github.com/apache/skywalking-go/protocols/skywalking.apache.org/|g' {} \;

echo "Removing original proto repository directory..."
rm -rf $LOCAL_REPO_DIR

echo "Code generation completed. Output directory: $OUTPUT_BASE_DIR"
