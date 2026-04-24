# Transfer Wormhole Progress Design

## Goal

Upgrade the transfer flow so it feels like one continuous premium bridge experience instead of a form followed by a generic progress card. The user should enter transfer details in a compact high-confidence card, then watch that same card transform into a cinematic wormhole journey once submission begins.

## Product Intent

The transfer surface should do two jobs well:

1. collect bridge inputs with clarity and confidence
2. turn live bridge progress into something memorable and spatially intuitive

The About page already explains the system deeply. The transfer page should not repeat that educational role. Instead, it should stay focused on the active transaction and make the progress feel alive, trustworthy, and easy to follow.

## Experience Model

The transfer flow becomes a two-state experience within the same card shell.

### State 1: Input card

- compact premium transfer card
- amount input
- destination selector
- recipient input
- wallet context
- single clear CTA

This is the calm setup state. It should feel precise, minimal, and expensive.

### State 2: Wormhole journey

After the user clicks `Bridge to Osmosis`, the same card morphs into a dark cinematic bridge-progress surface.

The transformation should feel like the transfer card itself has entered the bridge.

## Core Transition

The transition between input and progress should preserve continuity instead of swapping the whole page abruptly.

### Morph behavior

- the outer card shell remains in place
- the card expands and darkens into the wormhole scene
- form fields compress into a compact transfer manifest
- the wormhole visual grows out of the card body
- live stage tracking appears around or above the wormhole

This should read as a transformation, not a route change.

## Visual Centerpiece

The wormhole visual from the About page becomes the hero of the progress state, but its role changes.

### About page role

- exploratory
- educational
- hover and inspect

### Transfer page role

- live transaction progress
- focused
- stateful
- cinematic

The transfer version should be simpler and more decisive than the About version.

## Progress State Layout

The transformed progress card should have four parts.

### Surface scale

The progress state should feel dramatically larger than the input state.

- it should widen to nearly the full available landing width
- it should read more like a live bridge surface than a compact form card
- the user should feel a clear transition from "submitting a form" to "watching a cross-chain system work"

### 1. Compact transfer manifest

Pinned near the top-left of the transformed card.

Should show:

- amount
- destination
- recipient

This is a compressed memory of what the user submitted.

### 2. Live stage status

Pinned near the top-right of the transformed card.

Should show:

- current stage label
- short live sentence
- polling/live chip if applicable

This area should stay concise. It should not become a large content panel.

### 3. Wormhole hero

Centered and dominant.

Should include:

- Sepolia side
- AegisLink center
- Osmosis side
- active motion through the tunnel
- stage markers that communicate completed/current/upcoming state

### 4. Transaction proof area

Placed below or along the lower edge of the card.

Should show:

- source tx hash
- final Osmosis tx link once available
- reset/close action after completion

## Stage Model

The transfer page should use a focused live-progress stage model rather than the longer educational text model from About.

Recommended stages:

1. `Sepolia confirmed`
2. `Verifier checks`
3. `Bridge accounting`
4. `IBC handoff`
5. `Osmosis receipt`

## Stage Presentation Rules

### Current stage

- strongest highlight
- solid premium emerald block
- subtle green glow around it
- live motion bias in the wormhole
- short live copy

The current stage should be visually unmistakable. It should not look like a slightly brighter blue card. It should feel like the active bridge checkpoint has fully locked in.

### Completed stage

- softly resolved with muted green tint
- visible but quieter than the current stage
- should feel done, not disabled

### Upcoming stage

- dim and waiting
- still legible

## Stage Copy Style

Transfer progress should use short, live operational copy only.

Examples:

- `Sepolia deposit confirmed`
- `AegisLink is verifying bridge policy`
- `Value is credited inside the bridge zone`
- `IBC delivery to Osmosis is underway`
- `Osmosis receipt resolved`

This copy should feel active and grounded in the real transaction state, not educational or abstract.

## Mapping to Existing Bridge Status

The visual stage system should map cleanly onto the current backend/frontend session model.

### Suggested mapping

- `submitted` -> morph transition begins, source tx known
- `sepolia_confirmed` or equivalent early source state -> `Sepolia confirmed`
- `aegislink_processing` -> `Verifier checks` then `Bridge accounting`
- `osmosis_pending` -> `IBC handoff`
- `completed` -> `Osmosis receipt`
- `failed` -> progress freezes and the card enters a clear recovery/error state

If current status granularity is too coarse, the UI may need a small derived stage mapper rather than direct one-to-one status rendering.

## Motion Direction

Motion should be premium and restrained.

### Transition into progress

- field rows contract and fade into the manifest
- background shifts from light premium shell into deep-space treatment
- wormhole fades/expands in
- stage markers appear with staggered emphasis

### Active progress

- the wormhole carries a subtle moving pulse
- the active stage marker breathes or glows
- the tunnel color bias can shift based on active stage class

For the current-stage green treatment:

- the emerald block should remain stable and readable
- glow can animate softly, but the filled state is the main signal
- completed stages may keep a dimmer green memory so progression is easy to scan

### Completion

- the final stage resolves cleanly
- motion settles down
- receipt link becomes prominent

## Error Handling

The cinematic treatment must not hide problems.

If submission or polling fails:

- preserve the transformed card
- stop active tunnel motion
- show a clear compact error panel
- keep source metadata visible
- provide a direct retry/reset action where appropriate

Errors should feel contained and informative, not like the UI broke.

## Mobile Behavior

On mobile, the same morphing-card concept remains, but the layout compresses vertically.

- manifest moves above the wormhole
- live stage line sits below the manifest
- wormhole remains centered
- proof links move below the scene

The main priority is readability and perceived continuity, not rigid duplication of desktop layout.

## Accessibility

- motion must have a reduced-motion fallback
- stage state should remain understandable without animation
- source and destination hashes/links must remain readable and keyboard accessible
- the live stage area should be semantically clear for assistive tech

## Testing Expectations

Implementation should cover:

- input state still renders and validates as expected
- submitting transitions into the progress state
- live stage rendering changes with session status
- final destination link still appears on completion
- transfer metadata remains visible during progress

Visual fidelity is not a unit-test target, but state transitions and progress mapping must be covered.

## Success Criteria

The redesign is successful if:

- the transfer flow feels like one continuous bridge experience
- users can immediately tell where their transaction is in the journey
- the wormhole visual supports trust instead of acting like decoration
- the page remains more focused than the About experience while still feeling visually ambitious
