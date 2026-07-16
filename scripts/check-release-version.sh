#!/usr/bin/env bash
set -euo pipefail

version="${1:-}"
confirmation="${2:-}"

if [[ "$confirmation" != "RELEASE" ]]; then
  echo "Release abgebrochen: Bestätigung muss exakt RELEASE lauten." >&2
  exit 1
fi
if [[ ! "$version" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
  echo "Ungültige stabile Version: $version" >&2
  exit 1
fi

source_version="$(sed -n 's/^var appVersion = "\([0-9][0-9.]*\)-dev"$/\1/p' app.go)"
if [[ -z "$source_version" || "$version" != "$source_version" ]]; then
  echo "Release-Version $version stimmt nicht mit der Entwicklungsversion ${source_version:-unbekannt}-dev überein." >&2
  exit 1
fi
if git rev-parse --verify --quiet "refs/tags/v$version" >/dev/null; then
  echo "Tag v$version existiert bereits." >&2
  exit 1
fi

echo "Release v$version wurde ausdrücklich bestätigt und ist bereit zur Prüfung."
