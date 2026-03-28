#!/bin/bash
# Pull the latest release binary from GitHub
# Usage: ./scripts/pull-release.sh [--restart]

set -e

REPO="egg5233/arb_public"
BINARY="arb"
INSTALL_DIR="$(cd "$(dirname "$0")/.." && pwd)"

echo "Fetching latest release from $REPO..."
TAG=$(gh release view --repo "$REPO" --json tagName -q '.tagName' 2>/dev/null)
if [ -z "$TAG" ]; then
  echo "ERROR: Could not fetch latest release. Is gh authenticated?"
  exit 1
fi

echo "Latest release: $TAG"

# Download binary
echo "Downloading $BINARY..."
gh release download "$TAG" --repo "$REPO" --pattern "$BINARY" --dir /tmp --clobber

# Replace current binary
chmod +x "/tmp/$BINARY"
mv "/tmp/$BINARY" "$INSTALL_DIR/$BINARY"
echo "Installed to $INSTALL_DIR/$BINARY"

# Restart if requested
if [ "$1" = "--restart" ]; then
  echo "Restarting arb service..."
  sudo systemctl restart arb
  echo "Service restarted."
fi

echo "Done. Version: $TAG"
