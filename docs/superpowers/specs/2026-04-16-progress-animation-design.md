# Progress Animation Design

## Goal

Add a minimal, premium-feeling in-progress animation to the bridge session UI so users can tell that the transfer is actively progressing without introducing noisy loader patterns.

## Scope

- Update the in-progress bridge session surface in the frontend.
- Keep the motion black-and-white and visually restrained.
- Avoid generic spinners, large skeletons, or full-card movement.

## Approved Direction

Use two subtle motion cues together:

1. A live treatment in the current-stage hero card.
2. A softly animated active step in the status timeline.

## UI Behavior

### Hero card

- When the session is still in progress, show a soft live pulse next to the current stage.
- Add a faint sheen that moves slowly across the hero card.
- Stop the motion once the session is completed or failed.

### Timeline

- Completed steps remain solid and static.
- The current step gets a brighter label plus a breathing dot or ring.
- Future steps stay muted and static.

## Motion constraints

- Motion should be subtle and slow.
- Animation should not shift layout.
- Avoid multiple competing motion patterns.
- Respect reduced-motion preferences by falling back to static emphasis.

## Files

- `web/src/features/bridge/ProgressPanel.tsx`
- `web/src/styles/global.css`
- `web/src/features/bridge/bridge.test.tsx`

## Verification

- Frontend tests continue to pass.
- Build continues to pass.
- In-progress sessions show visible but restrained motion.
- Completed and failed sessions remain static.
