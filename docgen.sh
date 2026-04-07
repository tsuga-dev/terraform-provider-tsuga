#!/bin/bash
set -e
set -x

# Remove generated code first
rm -rf ./docs/

# Generate the documentation
cd tools && go generate ./...
