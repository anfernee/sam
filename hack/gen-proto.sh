#!/usr/bin/env bash
set -e

echo "Installing protobuf Go plugins..."
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest

echo "Generating Go protobuf code..."
mkdir -p api
protoc --go_out=paths=source_relative:. api/sam.proto

echo "Protobuf generation complete."
