#!/usr/bin/env bash
# Copy canonical agent skill into skills/kanbancli/ (must match internal/cli/SKILL.md).
set -euo pipefail
cd "$(dirname "$0")/.."
cp internal/cli/SKILL.md skills/kanbancli/SKILL.md
