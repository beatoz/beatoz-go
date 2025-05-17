#!/usr/bin/env bash
set -e

DRY=false
if [[ "$1" == "--dry" ]]; then
  DRY=true
fi

# 최신 태그 추출 (없으면 v0.0.0)
LATEST_TAG=$(git tag --sort=-v:refname | grep -E '^v[0-9]+\.[0-9]+\.[0-9]+$' | head -n 1)
[ -z "$LATEST_TAG" ] && LATEST_TAG="v0.0.0"

IFS='.' read -r MAJOR MINOR PATCH <<< "${LATEST_TAG#v}"

# 커밋 메시지 기반 변경 감지
COMMITS=$(git log "${LATEST_TAG}"..HEAD --pretty=format:"%s")

if echo "$COMMITS" | grep -q 'BREAKING CHANGE\|!:'
then
  ((MAJOR++)); MINOR=0; PATCH=0
elif echo "$COMMITS" | grep -q '^feat'
then
  ((MINOR++)); PATCH=0
elif echo "$COMMITS" | grep -q '^fix'
then
  ((PATCH++))
else
  echo "No version change detected."
  exit 0
fi

NEW_TAG="v$MAJOR.$MINOR.$PATCH"
echo "Calculated new tag: $NEW_TAG"

if $DRY; then
  echo "$NEW_TAG"
  exit 0
fi

# 태그 생성 및 푸시
git tag "$NEW_TAG"
git push origin "$NEW_TAG"
echo "New tag pushed: $NEW_TAG"