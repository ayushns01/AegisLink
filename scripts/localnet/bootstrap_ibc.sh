#!/usr/bin/env bash

set -euo pipefail

if [[ $# -lt 2 ]]; then
  echo "usage: $0 <aegislink-home> <destination-home>" >&2
  exit 1
fi

A_HOME="$1"
D_HOME="$2"

A_LINK="$A_HOME/data/ibc-link.json"
D_LINK="$D_HOME/data/ibc-link.json"

mkdir -p "$(dirname "$A_LINK")" "$(dirname "$D_LINK")"

cat >"$A_LINK" <<EOF
{
  "relay_mode": "hermes-local",
  "source_chain_id": "aegislink-sdk-1",
  "destination_chain_id": "osmo-local-1",
  "source_client_id": "07-tendermint-0",
  "destination_client_id": "07-tendermint-0",
  "connection_id": "connection-0",
  "source_port": "transfer",
  "source_channel": "channel-0",
  "destination_port": "transfer",
  "destination_channel": "channel-0"
}
EOF

cat >"$D_LINK" <<EOF
{
  "relay_mode": "hermes-local",
  "source_chain_id": "aegislink-sdk-1",
  "destination_chain_id": "osmo-local-1",
  "source_client_id": "07-tendermint-0",
  "destination_client_id": "07-tendermint-0",
  "connection_id": "connection-0",
  "source_port": "transfer",
  "source_channel": "channel-0",
  "destination_port": "transfer",
  "destination_channel": "channel-0"
}
EOF

cat <<EOF
Local Hermes-shaped IBC bootstrap ready.
- AegisLink home: $A_HOME
- Destination home: $D_HOME
- Relay path: route-relayer packet relay -> osmo-locald packet relay verbs
- Link metadata: $A_LINK and $D_LINK
EOF
