#!/usr/bin/env bash
set -e

LATEST_TAG=$(git tag --sort=-v:refname | grep -E '^v[0-9]+\.[0-9]+\.[0-9]+$' | head -n 1)
[ -z "$LATEST_TAG" ] && LATEST_TAG=""

if [ -n "$LATEST_TAG" ]; then
  VERSION=${LATEST_TAG#v}
  IFS='.' read -r MAJOR MINOR PATCH <<< "$VERSION"
  MAJOR=$(echo "${MAJOR:-0}" | tr -cd '0-9')
  MINOR=$(echo "${MINOR:-0}" | tr -cd '0-9')
  PATCH=$(echo "${PATCH:-0}" | tr -cd '0-9')
else
  MAJOR=0
  MINOR=0
  PATCH=0
fi

BASE=""
if [ -n "$LATEST_TAG" ]; then
  BASE=$(git merge-base "$LATEST_TAG" HEAD)
fi

COMMITS=$(git log ${BASE}..HEAD --pretty=format:"%s")

if echo "$COMMITS" | grep -q 'BREAKING CHANGE\|!:'; then
  MAJOR=$((MAJOR + 1))
  MINOR=0
  PATCH=0
elif echo "$COMMITS" | grep -q '^feat'; then
  MINOR=$((MINOR + 1))
  PATCH=0
elif echo "$COMMITS" | grep -q '^fix'; then
  PATCH=$((PATCH + 1))
else
  echo "No version change detected."
  exit 0
fi

NEW_TAG="v$MAJOR.$MINOR.$PATCH"
echo "$NEW_TAG"
