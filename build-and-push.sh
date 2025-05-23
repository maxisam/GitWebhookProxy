#!/bin/bash
set -e

# Print informational messages
echo "This script builds the GitWebhookProxy application and pushes it to Docker Hub."
echo "NOTE: This script is intended for local development and testing. The official CI/CD pipeline is managed by GitHub Actions."

# Build the builder image
echo "Building the builder image..."
docker build -t gitwebhookproxy-builder -f build/package/Dockerfile.build .

# Create a temporary directory
echo "Creating a temporary directory..."
mkdir -p ./tmp_build_output

# Extract build artifacts
echo "Extracting build artifacts..."
docker run --rm gitwebhookproxy-builder tar -cf - -C / Dockerfile.run GitWebhookProxy | tar -xf - -C ./tmp_build_output

# Prompt for Docker Hub details
read -p "Enter your Docker Hub username: " DOCKER_HUB_USERNAME
read -p "Enter the image name for Docker Hub (e.g., gitwebhookproxy): " IMAGE_NAME

# Determine Image Version
VERSION_TAG=$(git describe --tags --always --dirty 2>/dev/null)
if [ -z "$VERSION_TAG" ]; then
  echo "No Git tags found. Using 'latest' as the version."
  VERSION_TAG="latest"
fi
echo "Using version: $VERSION_TAG"

# Define image tags
LATEST_IMAGE_TAG="$DOCKER_HUB_USERNAME/$IMAGE_NAME:latest"
VERSIONED_IMAGE_TAG="$DOCKER_HUB_USERNAME/$IMAGE_NAME:$VERSION_TAG"

echo "Building final image with tags: $LATEST_IMAGE_TAG and $VERSIONED_IMAGE_TAG"
# Build the final image
docker build -t "$LATEST_IMAGE_TAG" -t "$VERSIONED_IMAGE_TAG" -f ./tmp_build_output/Dockerfile.run ./tmp_build_output

read -p "Do you want to push the image to Docker Hub? (y/n): " PUSH_TO_DOCKER_HUB
if [ "$PUSH_TO_DOCKER_HUB" != "y" ]; then
  echo "Skipping push to Docker Hub. The image is available locally as $LATEST_IMAGE_TAG and $VERSIONED_IMAGE_TAG."
  # Cleanup section
  echo "Cleaning up..."
  rm -rf ./tmp_build_output
  read -p "Do you want to remove the builder image (gitwebhookproxy-builder)? (y/n): " REMOVE_BUILDER_IMAGE
  if [ "$REMOVE_BUILDER_IMAGE" == "y" ]; then
    echo "Removing builder image..."
    docker rmi gitwebhookproxy-builder
  fi
  echo "Script completed. Image built locally."
  exit 0
fi

# Login to Docker Hub
echo "Logging in to Docker Hub..."
docker login -u "$DOCKER_HUB_USERNAME"
if [ $? -ne 0 ]; then
  echo "Docker login failed. Exiting."
  rm -rf ./tmp_build_output # Cleanup before exit
  exit 1
fi

# Push images to Docker Hub
echo "Pushing images to Docker Hub..."
docker push "$LATEST_IMAGE_TAG"
if [ $? -ne 0 ]; then
  echo "Failed to push $LATEST_IMAGE_TAG. Exiting."
  rm -rf ./tmp_build_output # Cleanup before exit
  exit 1
fi

docker push "$VERSIONED_IMAGE_TAG"
if [ $? -ne 0 ]; then
  echo "Failed to push $VERSIONED_IMAGE_TAG. Exiting."
  rm -rf ./tmp_build_output # Cleanup before exit
  exit 1
fi

# Cleanup
echo "Cleaning up..."
rm -rf ./tmp_build_output

read -p "Do you want to remove the builder image (gitwebhookproxy-builder)? (y/n): " REMOVE_BUILDER_IMAGE
if [ "$REMOVE_BUILDER_IMAGE" == "y" ]; then
  echo "Removing builder image..."
  docker rmi gitwebhookproxy-builder
fi

# Success message
echo "Script completed successfully. Images pushed to Docker Hub: $LATEST_IMAGE_TAG and $VERSIONED_IMAGE_TAG"
exit 0
