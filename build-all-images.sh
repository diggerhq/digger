#!/bin/bash
# Build all Docker images for Digger/Taco services
# 
# IMPORTANT: Images are built locally and NOT pushed automatically.
# You control where/if they get pushed.
#
# Usage:
#   ./build-all-images.sh [REGISTRY] [VERSION]
#
# Examples:
#   ./build-all-images.sh us-central1-docker.pkg.dev/my-project/digger v0.1.0  # GCP Artifact Registry (private)
#   ./build-all-images.sh ghcr.io/my-org v0.1.0                                # GitHub (can be private/public)
#   ./build-all-images.sh                                                      # Just tag locally, don't specify registry
#
set -e

# Configuration - YOU MUST SET YOUR REGISTRY!
if [ -z "$1" ]; then
    echo "ERROR: Registry not specified!"
    echo ""
    echo "Usage: $0 REGISTRY [VERSION]"
    echo ""
    echo "For GCP (private by default):"
    echo "  $0 REGION-docker.pkg.dev/PROJECT/REPO v0.1.0"
    echo ""
    echo "For GitHub Container Registry (set to private in settings):"
    echo "  $0 ghcr.io/YOUR_ORG v0.1.0"
    echo ""
    exit 1
fi

REGISTRY="${1}"
VERSION="${2:-v0.1.0}"
COMMIT_SHA=$(git rev-parse HEAD 2>/dev/null || echo "unknown")

# Colors for output
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

echo -e "${GREEN}╔════════════════════════════════════════╗${NC}"
echo -e "${GREEN}║   Building Digger/Taco Docker Images   ║${NC}"
echo -e "${GREEN}╚════════════════════════════════════════╝${NC}"
echo ""
echo -e "${YELLOW}Registry:${NC}  $REGISTRY"
echo -e "${YELLOW}Version:${NC}   $VERSION"
echo -e "${YELLOW}Commit:${NC}    $COMMIT_SHA"
echo -e "${YELLOW}Platform:${NC}  linux/amd64 (for GKE/GCP)"
echo ""
echo -e "${YELLOW}Note:${NC} Building on ARM64 Mac for AMD64 deployment"
echo -e "      This ensures images run correctly in GCP/GKE"
echo ""

# Build drift
echo -e "${YELLOW}[1/3] Building drift service...${NC}"
docker build \
  --platform linux/amd64 \
  -t ${REGISTRY}/drift:${VERSION} \
  -t ${REGISTRY}/drift:latest \
  --build-arg COMMIT_SHA=${COMMIT_SHA} \
  -f Dockerfile_drift \
  .
echo -e "${GREEN}✓ Drift built${NC}"
echo ""

# Build taco-statesman
echo -e "${YELLOW}[2/3] Building taco-statesman...${NC}"
cd taco
docker build \
  --platform linux/amd64 \
  -t ${REGISTRY}/taco-statesman:${VERSION} \
  -t ${REGISTRY}/taco-statesman:latest \
  --build-arg COMMIT_SHA=${COMMIT_SHA} \
  -f Dockerfile_statesman \
  .
cd ..
echo -e "${GREEN}✓ Taco-statesman built${NC}"
echo ""

# Build taco-ui (standalone Node.js SSR app - no Netlify!)
echo -e "${YELLOW}[3/3] Building taco-ui (Node.js + TanStack Start)...${NC}"
docker build \
  --platform linux/amd64 \
  -t ${REGISTRY}/taco-ui:${VERSION} \
  -t ${REGISTRY}/taco-ui:latest \
  --build-arg COMMIT_SHA=${COMMIT_SHA} \
  -f Dockerfile_ui \
  .
echo -e "${GREEN}✓ Taco-ui built (standalone Node.js, no Netlify dependencies)${NC}"
echo ""

echo -e "${GREEN}═══════════════════════════════════════${NC}"
echo -e "${GREEN}All images built successfully!${NC}"
echo -e "${GREEN}═══════════════════════════════════════${NC}"
echo ""
echo -e "${YELLOW}Built images:${NC}"
echo "  • ${REGISTRY}/drift:${VERSION}"
echo "  • ${REGISTRY}/taco-statesman:${VERSION}"
echo "  • ${REGISTRY}/taco-ui:${VERSION} (standalone Node.js + SSR)"
echo ""
echo -e "${YELLOW}Image Details:${NC}"
echo "  • drift:           Go-based drift detection service"
echo "  • taco-statesman:  Go-based IaC orchestration service"
echo "  • taco-ui:         React SSR app (TanStack Start, no Netlify)"
echo ""
echo -e "${YELLOW}IMPORTANT: Images are built locally only.${NC}"
echo -e "${YELLOW}They are NOT automatically pushed to any registry.${NC}"
echo ""
echo -e "${YELLOW}To push to your PRIVATE registry:${NC}"
echo ""
echo "  # First, authenticate (if not already done):"
echo "  gcloud auth configure-docker ${REGISTRY%%/*}  # For GCP"
echo "  # OR"
echo "  docker login ${REGISTRY%%/*}  # For other registries"
echo ""
echo "  # Then push:"
echo "  docker push ${REGISTRY}/drift:${VERSION}"
echo "  docker push ${REGISTRY}/drift:latest"
echo "  docker push ${REGISTRY}/taco-statesman:${VERSION}"
echo "  docker push ${REGISTRY}/taco-statesman:latest"
echo "  docker push ${REGISTRY}/taco-ui:${VERSION}"
echo "  docker push ${REGISTRY}/taco-ui:latest"
echo ""
echo -e "${YELLOW}Or push all at once:${NC}"
cat <<EOF
for img in drift taco-statesman taco-ui; do
  docker push ${REGISTRY}/\${img}:${VERSION}
  docker push ${REGISTRY}/\${img}:latest
done
EOF

