#!/usr/bin/env bash
# publish-npm.sh — 构建多平台二进制并发布 npm 包
# 用法: ./publish-npm.sh [patch|minor|major]  (默认 patch)
set -euo pipefail

BUMP=${1:-patch}
REPO_ROOT="$(cd "$(dirname "$0")" && pwd)"
NPM_DIR="$REPO_ROOT/npm"

# ── 平台矩阵 ──────────────────────────────────────────────
declare -A PLATFORMS=(
  ["linux-x64"]="linux amd64 x86_64-unknown-linux-musl"
  ["linux-arm64"]="linux arm64 aarch64-unknown-linux-musl"
  ["darwin-x64"]="darwin amd64 x86_64-apple-darwin"
  ["darwin-arm64"]="darwin arm64 aarch64-apple-darwin"
)

# ── 读取当前版本 ────────────────────────────────────────────
CURRENT=$(node -p "require('$NPM_DIR/package.json').version")
IFS='.' read -r MAJOR MINOR PATCH <<< "$CURRENT"
case "$BUMP" in
  major) NEW_VERSION="$((MAJOR+1)).0.0" ;;
  minor) NEW_VERSION="$MAJOR.$((MINOR+1)).0" ;;
  *)     NEW_VERSION="$MAJOR.$MINOR.$((PATCH+1))" ;;
esac

echo "📦 Publishing okp CLI: $CURRENT → $NEW_VERSION"

# ── 构建各平台二进制 ────────────────────────────────────────
for PKG_SUFFIX in "${!PLATFORMS[@]}"; do
  read -r GOOS GOARCH TRIPLE <<< "${PLATFORMS[$PKG_SUFFIX]}"
  PKG_DIR="$NPM_DIR/packages/okp-cli-$PKG_SUFFIX"
  BIN_DIR="$PKG_DIR/vendor/$TRIPLE/bin"
  mkdir -p "$BIN_DIR"

  BIN_NAME="okp"
  [[ "$GOOS" == "win32" ]] && BIN_NAME="okp.exe"

  echo "  🔨 $GOOS/$GOARCH → $PKG_SUFFIX"
  CGO_ENABLED=0 GOOS=$GOOS GOARCH=$GOARCH \
    go build -ldflags="-s -w -X main.version=$NEW_VERSION" \
    -o "$BIN_DIR/$BIN_NAME" \
    "$REPO_ROOT/cmd/cli"

  # 写子包 package.json
  cat > "$PKG_DIR/package.json" <<PKGJSON
{
  "name": "okp-cli-$PKG_SUFFIX",
  "version": "$NEW_VERSION",
  "description": "okp CLI binary for $(echo $PKG_SUFFIX | sed 's/-/ /')",
  "license": "UNLICENSED",
  "os": ["$(echo $PKG_SUFFIX | cut -d- -f1)"],
  "cpu": ["$(echo $PKG_SUFFIX | cut -d- -f2)"],
  "files": ["vendor"],
  "publishConfig": {"access": "public"}
}
PKGJSON
done

# ── 更新主包 version ────────────────────────────────────────
node -e "
  const fs = require('fs');
  const p = '$NPM_DIR/package.json';
  const pkg = JSON.parse(fs.readFileSync(p));
  pkg.version = '$NEW_VERSION';
  // 更新 optionalDependencies 版本
  for (const k of Object.keys(pkg.optionalDependencies || {})) {
    pkg.optionalDependencies[k] = '$NEW_VERSION';
  }
  fs.writeFileSync(p, JSON.stringify(pkg, null, 2) + '\n');
  console.log('  ✅ main package.json updated');
"

# ── Publish 子包 ────────────────────────────────────────────
for PKG_SUFFIX in "${!PLATFORMS[@]}"; do
  PKG_DIR="$NPM_DIR/packages/okp-cli-$PKG_SUFFIX"
  echo "  📤 publishing okp-cli-$PKG_SUFFIX@$NEW_VERSION"
  (cd "$PKG_DIR" && npm publish --access public)
done

# ── Publish 主包 ────────────────────────────────────────────
echo "  📤 publishing @markbangwu/okp@$NEW_VERSION"
(cd "$NPM_DIR" && npm publish --access public)

echo ""
echo "✅ Published @markbangwu/okp@$NEW_VERSION"
echo "   bun install -g @markbangwu/okp"
