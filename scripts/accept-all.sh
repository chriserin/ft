#!/usr/bin/env bash
set -euo pipefail

# Sets all no-activity scenarios to "accepted".
# Skips scenarios that already have a status.

./ft list --no-activity | while read -r line; do
  id=$(echo "$line" | grep -oP '@ft:\d+' | head -1)
  if [ -n "$id" ]; then
    ./ft status "$id" accepted
  fi
done
