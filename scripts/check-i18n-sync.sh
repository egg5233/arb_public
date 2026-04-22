#!/usr/bin/env bash
# check-i18n-sync.sh — verify web/src/i18n/en.ts and web/src/i18n/zh-TW.ts
# carry the exact same key set. Phase 9 lockstep gate invoked from every
# UI-touching plan's verify block. Exits 0 when in sync, non-zero on drift.
set -euo pipefail

EN_FILE="web/src/i18n/en.ts"
ZH_FILE="web/src/i18n/zh-TW.ts"

if [[ ! -f "$EN_FILE" || ! -f "$ZH_FILE" ]]; then
  echo "check-i18n-sync: missing locale file (en=$EN_FILE zh=$ZH_FILE)" >&2
  exit 2
fi

EN=$(mktemp)
ZH=$(mktemp)
trap 'rm -f "$EN" "$ZH"' EXIT

# Locale entries look like:  'nav.overview': 'Overview',
# Extract the dotted key between the leading quote and the closing quote
# followed by a colon. Tolerates either ' or " quoting.
extract_keys() {
  local file="$1"
  grep -oE "^[[:space:]]*['\"][A-Za-z0-9_.]+['\"][[:space:]]*:" "$file" \
    | sed -E "s/^[[:space:]]*['\"]//; s/['\"][[:space:]]*:[[:space:]]*$//" \
    | sort -u
}

extract_keys "$EN_FILE" > "$EN"
extract_keys "$ZH_FILE" > "$ZH"

DIFF=$(diff "$EN" "$ZH" || true)
if [[ -n "$DIFF" ]]; then
  echo "i18n key drift detected (en vs zh-TW):" >&2
  echo "$DIFF" >&2
  exit 1
fi

echo "i18n keys in sync: $(wc -l < "$EN") keys"
