#!/usr/bin/env bash
#
# check-nft-denoms.sh — scan x/nft denoms for records that fail ValidateGenesis
# (empty/whitespace Name, or Id failing the IBC-aware id rule), which brick
# export/import. Free-form Names (spaces/uppercase) are valid, not flagged.
#
# Usage:
#   ./scripts/check-nft-denoms.sh
#   NODE=tcp://host:26657 BIN=chain-maind ./scripts/check-nft-denoms.sh
#
# Requires: chain-maind (or $BIN) + jq.

set -euo pipefail

BIN="${BIN:-chain-maind}"
NODE="${NODE:-tcp://localhost:26657}"
PAGE_LIMIT=100

command -v "$BIN" >/dev/null || { echo "error: $BIN not found in PATH" >&2; exit 1; }
command -v jq >/dev/null || { echo "error: jq not found in PATH" >&2; exit 1; }

# Mirror types.ValidateDenomName: non-empty after trimming whitespace.
name_ok() {
  local n="${1//[$' \t\n\r']/}"
  [[ -n "$n" ]]
}

# Mirror types.ValidateDenomIDWithIBC.
id_ok() {
  local id="$1"
  if [[ "$id" == ibc/* ]]; then
    [[ ${#id} -eq 68 && "${id#ibc/}" =~ ^[0-9a-fA-F]{64}$ ]]   # "ibc/" + 64 hex
  else
    [[ ${#id} -ge 3 && ${#id} -le 64 && "$id" =~ ^[a-z][a-z0-9]*$ ]]
  fi
}

echo "Scanning NFT denoms via $BIN (node $NODE)..." >&2

next_key=""
total=0
bad=0
while :; do
  if [[ -n "$next_key" ]]; then
    page=$("$BIN" query nft denoms --node "$NODE" --output json \
      --page-key "$next_key" --limit "$PAGE_LIMIT")
  else
    page=$("$BIN" query nft denoms --node "$NODE" --output json \
      --limit "$PAGE_LIMIT")
  fi

  while IFS=$'\t' read -r id name; do
    [[ -z "$id" && -z "$name" ]] && continue
    total=$((total + 1))
    reasons=""
    name_ok "$name" || reasons="empty-name"
    id_ok "$id"     || reasons="${reasons:+$reasons,}invalid-id"
    if [[ -n "$reasons" ]]; then
      bad=$((bad + 1))
      printf 'POISONED [%s] id=%q name=%q\n' "$reasons" "$id" "$name"
    fi
  done < <(echo "$page" | jq -r '.denoms[]? | [.id, .name] | @tsv')

  next_key=$(echo "$page" | jq -r '.pagination.next_key // empty')
  [[ -z "$next_key" || "$next_key" == "null" ]] && break
done

echo "---" >&2
echo "scanned=$total poisoned=$bad" >&2
[[ "$bad" -eq 0 ]] || exit 2
