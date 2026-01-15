#!/bin/bash
# WolGoWeb Enhanced - Multi-architecture Docker Build Script
# Update BUILD_PUSH to your Docker Hub username

BUILD_NAME="buildx-wol-go-web"
BUILD_PLAT=linux/amd64,linux/arm64,linux/arm/v7
BUILD_PUSH=zy1234567/wol-go-web-ui  # Change this to your Docker Hub repo
BUILD_VERSION=$(cat ../.version 2>/dev/null || echo "latest")

echo "=========== Building ${BUILD_NAME} ==========="
echo "Version: ${BUILD_VERSION}"
echo "Platforms: ${BUILD_PLAT}"
echo "Push to: ${BUILD_PUSH}"
echo ""

# Create and use buildx builder
docker buildx create --name ${BUILD_NAME} --use 2>/dev/null || docker buildx use ${BUILD_NAME}

# Build and push
docker buildx build \
    --platform=${BUILD_PLAT} \
    --tag ${BUILD_PUSH}:${BUILD_VERSION} \
    --tag ${BUILD_PUSH}:latest \
    --build-arg BUILD_VERSION=${BUILD_VERSION} \
    --progress=plain \
    --push \
    -f dockerfile \
    ..

# Cleanup
docker buildx rm ${BUILD_NAME}

echo ""
echo "✅ Multiarch build complete!"
echo "   ${BUILD_PUSH}:latest"
echo "   ${BUILD_PUSH}:${BUILD_VERSION}"