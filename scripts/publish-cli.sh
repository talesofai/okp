#!/usr/bin/env bash
# Build multi-platform CLI binaries and publish npm packages.
# Usage:
#   ./scripts/publish-cli.sh <version>
#   ./scripts/publish-cli.sh patch|minor|major   # bump from npm/package.json
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
NPM_DIR="$REPO_ROOT/npm"
ARG=${1:-}

declare -A PLATFORMS=(
  ["linux-x64"]="linux amd64 x86_64-unknown-linux-musl"
  ["linux-arm64"]="linux arm64 aarch64-unknown-linux-musl"
  ["darwin-x64"]="darwin amd64 x86_64-apple-darwin"
  ["darwin-arm64"]="darwin arm64 aarch64-apple-darwin"
)

CURRENT=$(node -p "require('$NPM_DIR/package.json').version")
if [[ -z "$ARG" ]]; then
  echo "usage: $0 <version|patch|minor|major>" >&2
  exit 1
fi

case "$ARG" in
  major|minor|patch)
    IFS='.' read -r MAJOR MINOR PATCH <<< "$CURRENT"
    case "$ARG" in
      major) NEW_VERSION="$((MAJOR + 1)).0.0" ;;
      minor) NEW_VERSION="$MAJOR.$((MINOR + 1)).0" ;;
      patch) NEW_VERSION="$MAJOR.$MINOR.$((PATCH + 1))" ;;
    esac
    ;;
  *)
    NEW_VERSION="$ARG"
    ;;
esac

if [[ ! "$NEW_VERSION" =~ ^[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z.-]+)?$ ]]; then
  echo "invalid version: $NEW_VERSION" >&2
  exit 1
fi

echo "Publishing okp CLI: $CURRENT → $NEW_VERSION"

for PKG_SUFFIX in "${!PLATFORMS[@]}"; do
  read -r GOOS GOARCH TRIPLE <<< "${PLATFORMS[$PKG_SUFFIX]}"
  PKG_DIR="$NPM_DIR/packages/okp-cli-$PKG_SUFFIX"
  BIN_DIR="$PKG_DIR/vendor/$TRIPLE/bin"
  mkdir -p "$BIN_DIR"

  echo "  build $GOOS/$GOARCH → $PKG_SUFFIX"
  CGO_ENABLED=0 GOOS=$GOOS GOARCH=$GOARCH \
    go build -ldflags="-s -w -X main.version=$NEW_VERSION" \
    -o "$BIN_DIR/okp" \
    "$REPO_ROOT/cmd/cli"

  cat > "$PKG_DIR/package.json" <<PKGJSON
{
  "name": "okp-cli-$PKG_SUFFIX",
  "version": "$NEW_VERSION",
  "description": "okp CLI binary for ${PKG_SUFFIX//-/ }",
  "license": "MIT",
  "os": ["${PKG_SUFFIX%-*}"],
  "cpu": ["${PKG_SUFFIX#*-}"],
  "files": ["vendor"],
  "publishConfig": {
    "access": "public",
    "provenance": true
  },
  "repository": {
    "type": "git",
    "url": "https://github.com/talesofai/okp.git",
    "directory": "npm/packages/okp-cli-$PKG_SUFFIX"
  }
}
PKGJSON
done

node -e "
  const fs = require('fs');
  const p = '$NPM_DIR/package.json';
  const pkg = JSON.parse(fs.readFileSync(p, 'utf8'));
  pkg.version = '$NEW_VERSION';
  pkg.license = 'MIT';
  pkg.repository = {
    type: 'git',
    url: 'https://github.com/talesofai/okp.git',
    directory: 'npm',
  };
  pkg.publishConfig = Object.assign({}, pkg.publishConfig, {
    access: 'public',
    provenance: true,
  });
  for (const k of Object.keys(pkg.optionalDependencies || {})) {
    pkg.optionalDependencies[k] = '$NEW_VERSION';
  }
  fs.writeFileSync(p, JSON.stringify(pkg, null, 2) + '\n');
  console.log('  main package.json updated');
"

# Prefer npm Trusted Publishing (OIDC). If a classic token is present, npm may
# use it instead — clear empty placeholders that break OIDC.
if [[ -z "${NODE_AUTH_TOKEN:-}" || "${NODE_AUTH_TOKEN}" == "XXXXX-XXXXX-XXXXX-XXXXX" ]]; then
  unset NODE_AUTH_TOKEN || true
fi
if [[ -z "${NPM_TOKEN:-}" ]]; then
  unset NPM_TOKEN || true
fi
npm config delete //registry.npmjs.org/:_authToken >/dev/null 2>&1 || true

publish_one() {
  local dir="$1"
  # --access public for first publish of scoped packages; unscoped is public by default
  if [[ -n "${NPM_TOKEN:-}${NODE_AUTH_TOKEN:-}" ]]; then
    (cd "$dir" && npm publish --access public --provenance)
  else
    # OIDC trusted publishing path
    (cd "$dir" && npm publish --access public --provenance)
  fi
}

for PKG_SUFFIX in "${!PLATFORMS[@]}"; do
  PKG_DIR="$NPM_DIR/packages/okp-cli-$PKG_SUFFIX"
  echo "  publish okp-cli-$PKG_SUFFIX@$NEW_VERSION"
  publish_one "$PKG_DIR"
done

echo "  publish @markbangwu/okp@$NEW_VERSION"
publish_one "$NPM_DIR"

echo ""
echo "Published @markbangwu/okp@$NEW_VERSION"
echo "  npm install -g @markbangwu/okp"
