#!/bin/sh
# Checks that README, docs and CHANGELOG are ready for release <version>.
# Run by the release workflow before publishing; see "Releasing" in CHANGELOG.md.
set -u

version="${1:?usage: $0 <version>}"
failed=0

markers=$(grep -n -x '\*(unreleased)\*' README.md docs/*.md)
if [ -n "$markers" ]; then
	echo "Replace these markers with *(since $version)*:"
	echo "$markers"
	failed=1
fi

if ! grep -q -E "^## $(printf '%s' "$version" | sed 's/\./\\./g')( |$)" CHANGELOG.md; then
	echo "CHANGELOG.md has no section '## $version'."
	failed=1
fi

if ! head -n 5 README.md | grep -F "/blob/$version/README.md" | grep -q -F "**$version**"; then
	echo "The note at the top of README.md does not point to release $version."
	failed=1
fi

if [ "$failed" -ne 0 ]; then
	echo "Docs are not ready for release $version."
	exit 1
fi
echo "Docs are ready for release $version."
