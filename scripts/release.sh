#!/bin/bash

# minfin Release Automation Script
# Usage: ./scripts/release.sh <version>
# Example: ./scripts/release.sh 0.1.0

set -e

VERSION=$1

if [ -z "$VERSION" ]; then
    echo "Usage: $0 <version> (e.g., 0.1.0)"
    exit 1
fi

# Normalize: accept either "0.1.0" or "v0.1.0", work with the bare version.
VERSION="${VERSION#v}"
TAG="v$VERSION"
REPO_ROOT=$(git rev-parse --show-toplevel)
DIST="$REPO_ROOT/dist"

LINUX_DESKTOP="$DIST/minfin-desktop-$VERSION-linux-amd64"
WIN_DESKTOP="$DIST/minfin-desktop-$VERSION-windows-amd64.exe"
LINUX_GTK="$DIST/minfin-gtk-$VERSION-linux-amd64"

# 1. Validation
if ! command -v gh &> /dev/null; then
    echo "Error: 'gh' (GitHub CLI) is not installed."
    exit 1
fi

if ! git diff-index --quiet HEAD --; then
    echo "Error: You have uncommitted changes. Please commit or stash them first."
    exit 1
fi

CURRENT_BRANCH=$(git branch --show-current)
if [ "$CURRENT_BRANCH" != "main" ]; then
    echo "Error: You are on branch '$CURRENT_BRANCH'. Releases must be performed from 'main'."
    exit 1
fi

echo "🚀 Starting release process for $TAG..."

# 2. Tag and Push
echo "🏷️  Tagging $TAG..."
if git rev-parse "$TAG" >/dev/null 2>&1; then
    echo "Warning: Tag $TAG already exists locally."
else
    git tag -a "$TAG" -m "Release $TAG"
fi
git push origin main
git push origin "$TAG"

# 3. Build binaries into a clean dist/
echo "📦 Building binaries..."
make -C "$REPO_ROOT" clean
mkdir -p "$DIST"

# Desktop app: pure Go, cross-compiled, no CGO.
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -o "$LINUX_DESKTOP" ./cmd/minfin-desktop
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 \
    go build -ldflags "-H=windowsgui" -o "$WIN_DESKTOP" ./cmd/minfin-desktop

# GTK app: native build (CGO + GTK4/libadwaita); dynamically linked, so users
# need those libs installed. Built for the host arch.
go build -o "$LINUX_GTK" ./cmd/minfin-gtk

# 4. Create GitHub Release
echo "🌐 Creating GitHub Release..."
gh release create "$TAG" \
    "$LINUX_DESKTOP" "$WIN_DESKTOP" "$LINUX_GTK" \
    --title "Release $TAG" --generate-notes

# 5. Bump the Homebrew formula to point at this tag's source tarball.
# Homebrew reads the formula from main (the tap), not from inside the tarball,
# so committing this after the tag is fine.
echo "🍺 Updating Homebrew formula..."
FORMULA="$REPO_ROOT/Formula/minfin.rb"
TARBALL_URL="https://github.com/zackb/minfin/archive/refs/tags/$TAG.tar.gz"
SHA=$(curl -sL "$TARBALL_URL" | sha256sum | cut -d' ' -f1)
sed -i \
    -e "s|archive/refs/tags/v[0-9.]*\.tar\.gz|archive/refs/tags/$TAG.tar.gz|" \
    -e "s|sha256 \"[a-f0-9]*\"|sha256 \"$SHA\"|" \
    "$FORMULA"
git add "$FORMULA"
git commit -m "brew: bump formula to $TAG"
git push origin main

# TODO: AUR (minfin-git / minfin-bin) add later.

echo "✅ Release $VERSION deployed to GitHub!"
