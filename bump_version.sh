#!/bin/bash

set -e

# 1. Fetch latest tags from remote
git fetch --tags

# 2. Get latest tag
latest_tag=$(git describe --tags "$(git rev-list --tags --max-count=1)" 2>/dev/null || echo "0.0.0")
echo "Latest tag: $latest_tag"

# 3. Ask for version bump type
echo "Choose version bump type:"
echo "1) patch"
echo "2) minor"
echo "3) major"
read -p "Enter 1, 2, or 3: " choice

# 4. Split version into parts
IFS='.' read -r major minor patch <<< "${latest_tag//v/}"

# 5. Increment version
case $choice in
  1)
    patch=$((patch + 1))
    ;;
  2)
    minor=$((minor + 1))
    patch=0
    ;;
  3)
    major=$((major + 1))
    minor=0
    patch=0
    ;;
  *)
    echo "Invalid choice"
    exit 1
    ;;
esac

new_tag="v$major.$minor.$patch"
echo "New tag will be: $new_tag"

# 6. Create the tag
git tag $new_tag

# 7. Push the tag to remote
git push origin $new_tag

echo "Tag $new_tag pushed successfully!"
