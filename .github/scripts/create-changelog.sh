#!/usr/bin/env bash
set -e

NEW_TAG="$1"
LAST_TAG="$2"

if [[ -z "$NEW_TAG" || -z "$LAST_TAG" ]]; then
  echo "Usage: $0 <new_tag> <last_tag>"
  exit 1
fi

BASE=$(git merge-base "$LAST_TAG" HEAD)
COMMITS=$(git log "$BASE"..HEAD --pretty=format:"%s")

echo "## $NEW_TAG - $(date +%Y-%m-%d)" > CHANGELOG.md
echo "" >> CHANGELOG.md

declare -A TYPE_LABELS=(
  [feat]="### Features"
  [fix]="### Bug Fixes"
  [docs]="### Documentation"
  [chore]="### Other Changes"
  [refactor]="### Refactoring"
  [test]="### Tests"
  [perf]="### Performance"
)

for TYPE in "${!TYPE_LABELS[@]}"; do
  LOG=$(echo "$COMMITS" | grep "^$TYPE" || true)
  if [[ -n "$LOG" ]]; then
    echo "${TYPE_LABELS[$TYPE]}" >> CHANGELOG.md
    echo "$LOG" >> CHANGELOG.md
    echo "" >> CHANGELOG.md
  fi
done