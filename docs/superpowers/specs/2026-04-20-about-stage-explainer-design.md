# About Stage Explainer Design

## Goal

Upgrade the About page so the bridge stages explain themselves deeply without pushing the user into a detached sidebar. The wormhole remains the visual hero, while each stage marker becomes the place where explanation appears.

The desired outcome is a page that feels cinematic first, educational second, and documentation-backed throughout.

## Product Intent

The About page should answer two questions at the same time:

1. What is happening as value moves from Sepolia to Osmosis?
2. Why should a user trust that each step is real and meaningful?

The current inspector model explains the flow, but it asks the user to divide attention between the stage marker and a separate content panel. The new model should make the explanation feel spatially attached to the bridge itself.

## Core Interaction

Each stage marker around the wormhole becomes a compact interactive node.

### Collapsed state

- The node is small, dark blue, and glassy.
- It should feel closer to a navigational star or systems beacon than a pill-shaped card.
- The collapsed state shows only the stage number and a short title.
- The marker should stay visually secondary to the wormhole.

### Expanded state

- Hovering a stage expands a directly attached explanation capsule.
- Clicking a stage pins the capsule open.
- Only one stage can be expanded at a time.
- Hover should temporarily preview a stage unless another stage is pinned.
- On mobile, only tap-to-open should be used.

### Attachment model

- The explanation capsule should emerge from the marker itself.
- The motion should feel like a bloom, unfold, or reveal from the node rather than a tooltip popping above the scene.
- The capsule should stay physically tied to the marker using a subtle anchor stem or glow trail.

## Content Model Per Stage

Each expanded capsule should contain layered information in a consistent order.

### Layer 1: What is happening now

A concise explanation of the visible bridge step in user-facing language.

### Layer 2: Inside AegisLink

A deeper protocol-facing explanation of what the bridge is doing internally.

### Layer 3: Why this matters

A short trust or systems explanation that answers why this step exists and what it guarantees.

### Footer row

A tiny systems strip should reinforce the context for the stage. This can include a combination of:

- source chain
- bridge boundary
- delivery phase
- destination effect

The footer must remain compact and scannable.

## Recommended Stage Copy Shape

### Deposit signed

- What is happening now: the user signs and broadcasts the Sepolia deposit transaction.
- Inside AegisLink: the bridge session begins from a real source transaction identity.
- Why this matters: the transfer now has a concrete origin that can be tracked and verified.

### Verifier checks

- What is happening now: AegisLink validates deposit evidence and replay safety.
- Inside AegisLink: signer policy, verifier logic, and bridge policy decide whether the transfer can advance.
- Why this matters: invalid or duplicated deposits do not contaminate bridge accounting.

### Bridge accounting

- What is happening now: verified value is credited inside the bridge zone.
- Inside AegisLink: source confirmation becomes internal bridge-owned state.
- Why this matters: the bridge now controls the outbound delivery leg instead of relying on the source chain alone.

### IBC handoff

- What is happening now: AegisLink initiates the outbound route toward Osmosis.
- Inside AegisLink: the route, timeout policy, and packet state are created.
- Why this matters: this is the moment where the system turns verification into cross-chain delivery.

### Osmosis receipt

- What is happening now: the transfer lands on Osmosis and resolves to a destination transaction.
- Inside AegisLink: the bridge session is matched back to the final receipt.
- Why this matters: the user can verify settlement directly instead of trusting a vague completion label.

## Visual Direction

The About page should keep the side-view wormhole in space as the centerpiece.

### Wormhole scene

- Left portal represents Sepolia.
- Right portal represents Osmosis.
- AegisLink remains centered in the tunnel.
- The tunnel should feel active and dimensional, not flat.

### Stage nodes

- Use dark blue or blue-black tones that match the scene.
- Avoid bright white cards.
- Keep them materially consistent with the space theme.
- Active nodes can brighten slightly, but should still feel integrated into the environment.

### Expanded capsules

- Use layered glass, low-contrast borders, and directional glows.
- Color accents can reflect stage class:
  - pink for source
  - violet for verification/accounting
  - gold for delivery/receipt
- Expanded capsules should feel premium and restrained, not game-like or neon-heavy.

## Motion

Motion should support understanding rather than distract from it.

### Marker behavior

- Idle markers can have a low-amplitude shimmer or soft pulse.
- On hover, the marker should brighten and the capsule should unfold in under 220ms.
- On click, the open state should settle more firmly than hover.

### Tunnel response

- When a stage is active, the tunnel or bridge pulse should subtly bias toward that stage.
- Examples:
  - source-side glow for deposit
  - center pulse for verification and accounting
  - destination-side travel emphasis for IBC and receipt

### Reduced motion

- All decorative motion must degrade cleanly for reduced-motion users.
- The information hierarchy must still be fully understandable with motion disabled.

## Layout Behavior

### Desktop

- The wormhole scene is the hero.
- Expanded stage capsules float near their anchor positions without obscuring the core scene.
- Documentation cards remain below the visual story.

### Mobile

- The scene becomes vertically compressed.
- Stage nodes remain tappable and legible.
- Expanded capsules may shift below the scene if spatial overlap becomes too tight.
- The interaction must prioritize readability over strict positional purity.

## Documentation Relationship

The docs section remains below the wormhole story and should feel like proof surfaces behind the visual explanation.

The stage interaction explains the system conceptually. The documentation cards prove there is real architecture and operational depth behind the narrative.

## Accessibility

- All stage nodes must be keyboard reachable.
- Focus state must be explicit and attractive.
- Hover-only access cannot be the only way to reveal explanation.
- The expanded capsule must be readable by screen readers in stage order.
- The active stage should expose pressed or selected state semantics.

## Testing Expectations

Implementation should include coverage for:

- opening the About page from the AegisLink menu
- rendering stage nodes on the standalone About page
- updating the active stage content on click
- preserving a meaningful default active stage
- documentation links remaining present on the About page

Visual behavior such as motion and exact positioning does not need snapshot-level tests, but interaction state and core copy updates should be covered.

## Success Criteria

The redesign is successful if:

- the user understands each stage without hunting for meaning elsewhere
- the explanation feels attached to the bridge diagram rather than detached from it
- the page looks premium and cinematic without becoming cluttered
- the About page still feels credible to technical readers because the copy reflects real bridge behavior
