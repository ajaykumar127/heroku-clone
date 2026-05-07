#!/bin/bash
# Build script using Cloud Native Buildpacks (pack CLI)
# This demonstrates how to use CNB to build applications

set -e

APP_NAME=$1
COMMIT=$2
REPO_PATH=$3

if [ -z "$APP_NAME" ] || [ -z "$COMMIT" ] || [ -z "$REPO_PATH" ]; then
    echo "Usage: $0 <app-name> <commit> <repo-path>"
    exit 1
fi

BUILD_DIR="/tmp/builds/${APP_NAME}-${COMMIT}"
IMAGE_NAME="localhost:5000/${APP_NAME}:${COMMIT}"

echo "=====> Building application: $APP_NAME"
echo "       Commit: $COMMIT"
echo "       Repo: $REPO_PATH"
echo ""

# Clone repo to build directory
echo "-----> Cloning repository"
rm -rf "$BUILD_DIR"
git clone "$REPO_PATH" "$BUILD_DIR"
cd "$BUILD_DIR"
git checkout "$COMMIT" 2>/dev/null || git checkout main

# Detect application type
echo "-----> Detecting application type"
if [ -f "package.json" ]; then
    BUILDER="paketobuildpacks/builder:base"
    echo "       Detected: Node.js"
elif [ -f "requirements.txt" ] || [ -f "Pipfile" ]; then
    BUILDER="paketobuildpacks/builder:base"
    echo "       Detected: Python"
elif [ -f "Gemfile" ]; then
    BUILDER="paketobuildpacks/builder:base"
    echo "       Detected: Ruby"
elif [ -f "go.mod" ]; then
    BUILDER="paketobuildpacks/builder:base"
    echo "       Detected: Go"
elif [ -f "pom.xml" ] || [ -f "build.gradle" ]; then
    BUILDER="paketobuildpacks/builder:base"
    echo "       Detected: Java"
elif [ -f "Dockerfile" ]; then
    echo "       Detected: Dockerfile"
    echo "-----> Building from Dockerfile"
    docker build -t "$IMAGE_NAME" .
    docker push "$IMAGE_NAME"
    echo "=====> Build complete!"
    exit 0
else
    echo "       ERROR: Unable to detect application type"
    exit 1
fi

# Build with pack CLI (Cloud Native Buildpacks)
echo "-----> Building with buildpacks"
pack build "$IMAGE_NAME" \
    --builder "$BUILDER" \
    --path "$BUILD_DIR" \
    --publish

echo ""
echo "=====> Build complete!"
echo "       Image: $IMAGE_NAME"
echo ""

# Cleanup
rm -rf "$BUILD_DIR"
