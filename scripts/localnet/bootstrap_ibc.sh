#!/usr/bin/env bash

set -euo pipefail

if [[ $# -lt 2 ]]; then
  echo "usage: $0 <aegislink-home> <destination-home>" >&2
  exit 1
fi

A_HOME="$1"
D_HOME="$2"

cat <<EOF
Local IBC-style bootstrap ready.
- AegisLink home: $A_HOME
- Destination home: $D_HOME
- Relay path: route-relayer command target -> osmo-locald
EOF
