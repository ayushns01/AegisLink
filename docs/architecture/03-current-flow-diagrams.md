# AegisLink Current Flow Diagrams

These diagrams describe the repository as it exists today, not the long-term roadmap.

## End-to-end local bridge flow

```mermaid
sequenceDiagram
    participant U as User
    participant E as Ethereum Gateway
    participant BR as Bridge Relayer
    participant A as AegisLink Runtime
    participant RR as Route Relayer
    participant O as Osmosis-lite Target

    U->>E: Deposit asset
    E-->>BR: Deposit event over local RPC
    BR->>A: Submit attested claim
    A->>A: Verify claim, limits, pause, replay
    A->>A: Mint bridged balance
    A->>RR: Expose pending routed transfer
    RR->>O: Deliver packet-shaped route
    O->>O: Persist packet, execute credit or swap
    O-->>RR: Acknowledgement becomes ready later
    RR->>A: Mark completed, failed, or timed out
    A->>A: Preserve refund-safe state when needed
```

## Destination route lifecycle

```mermaid
stateDiagram-v2
    [*] --> pending_on_aegislink
    pending_on_aegislink --> received: route-relayer delivers packet
    received --> executed: destination executes credit or swap
    received --> ack_ready: timeout before execution
    executed --> ack_ready: destination result recorded
    ack_ready --> ack_relayed: route-relayer consumes acknowledgement
    ack_relayed --> completed_on_aegislink: success acknowledgement
    ack_relayed --> ack_failed_on_aegislink: execution failure acknowledgement
    ack_relayed --> timed_out_on_aegislink: timeout acknowledgement
    timed_out_on_aegislink --> refunded_on_aegislink: refund recovery
```

## Read these with the right lens

- `pending_on_aegislink` lives on the AegisLink side.
- `received`, `executed`, `ack_ready`, and `ack_relayed` live on the destination-side harness.
- `completed_on_aegislink`, `ack_failed_on_aegislink`, and `timed_out_on_aegislink` are the source-side route outcomes after acknowledgement processing.
- This is intentionally a strong local harness, not a claim that the repository already has live IBC, CometBFT, or a full Osmosis deployment.
