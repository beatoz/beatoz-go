#!/usr/bin/env bash
set -e

NEW_TAG="$1"
LAST_TAG="$2"

if [[ -z "$NEW_TAG" || -z "$LAST_TAG" ]]; then
  echo "Usage: $0 <new_tag> <last_tag>"
  exit 1
fi

echo -e "## $NEW_TAG - $(date +%Y-%m-%d)\n" >> CHANGELOG.md

TYPES=("feat" "fix" "chore" "docs" "refactor" "test" "perf")
for type in "${TYPES[@]}"; do
  entries=$(git log "$LAST_TAG..HEAD" --grep="^$type" --pretty=format:"- %s")
  if [ -n "$entries" ]; then
    echo -e "### $(case $type in
      feat) echo 'Features' ;;
      fix) echo 'Bug Fixes' ;;
      chore) echo 'Other Changes' ;;
      docs) echo 'Documentation' ;;
      refactor) echo 'Code Refactoring' ;;
      test) echo 'Tests' ;;
      perf) echo 'Performance Improvements' ;;
    esac)" >> CHANGELOG.md
    echo "$entries" >> CHANGELOG.md
    echo "" >> CHANGELOG.md
  fi
done