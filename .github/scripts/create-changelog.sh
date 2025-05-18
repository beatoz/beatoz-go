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

# 대상 타입들
TYPES=(feat fix docs chore refactor test perf)

for TYPE in "${TYPES[@]}"; do
  SECTION_TITLE=""
  case "$TYPE" in
    feat) SECTION_TITLE="### Features" ;;
    fix) SECTION_TITLE="### Bug Fixes" ;;
    docs) SECTION_TITLE="### Documentation" ;;
    chore) SECTION_TITLE="### Other Changes" ;;
    refactor) SECTION_TITLE="### Refactoring" ;;
    test) SECTION_TITLE="### Tests" ;;
    perf) SECTION_TITLE="### Performance" ;;
  esac

  LOG=$(echo "$COMMITS" | grep "^$TYPE" || true)

  if [[ -n "$LOG" ]]; then
    echo "$SECTION_TITLE" >> CHANGELOG.md
    echo "$LOG" | sed 's/^/- /' >> CHANGELOG.md
    echo "" >> CHANGELOG.md
  fi
done