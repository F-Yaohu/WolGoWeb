#!/bin/bash
# Local Docker build script (without push)

IMAGE_NAME="wol-go-web"
BUILD_VERSION=$(cat ../.version 2>/dev/null || echo "latest")

echo "🔨 Building Docker image locally..."
echo "Image: ${IMAGE_NAME}:${BUILD_VERSION}"
echo ""

cd .. && docker build \
    -f docker/dockerfile \
    -t ${IMAGE_NAME}:${BUILD_VERSION} \
    -t ${IMAGE_NAME}:latest \
    --build-arg BUILD_VERSION=${BUILD_VERSION} \
    .

if [ $? -eq 0 ]; then
    echo ""
    echo "✅ Build successful!"
    echo ""
    echo "Run with:"
    echo "  docker run -d --net=host ${IMAGE_NAME}:latest"
    echo ""
    echo "Or use docker-compose.yml and change image name to:"
    echo "  image: ${IMAGE_NAME}:latest"
else
    echo ""
    echo "❌ Build failed!"
    exit 1
fi
