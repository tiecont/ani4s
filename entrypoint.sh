#!/bin/bash

# Exit immediately if a command exits with a non-zero status
set -e

# Ensure Air is installed, not use this for production. Before push build
# if ! command -v air &> /dev/null; then
#   echo "Error: Air is not installed"
#   exit 1
# fi

# Ensure the main binary is built before running
if [ ! -f "./bin/main" ]; then
  echo "Binary './bin/main' not found. Building it now..."
  go build -buildvcs=false -o ./bin/main .
fi

# Start the application with Air
echo "Starting the application with Air..."
exec air