#!/usr/bin/env bash
# Compatibility wrapper. Prefer: ./scripts/publish-cli.sh
set -euo pipefail
ROOT="$(cd "$(dirname "$0")" && pwd)"
exec "$ROOT/scripts/publish-cli.sh" "${1:-patch}"
