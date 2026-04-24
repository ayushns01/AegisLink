# About Stage Explainer Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the detached About-page inspector with stage-attached expandable explainer capsules that teach each bridge step directly from the wormhole diagram.

**Architecture:** Keep the existing standalone About page route in the landing shell, but refactor the About scene so each stage node owns its own explanatory content. The React component should manage one active stage plus pinned state, while CSS handles the capsule reveal, anchor styling, and restrained motion.

**Tech Stack:** React, TypeScript, Vitest, Testing Library, Vite CSS

---

### Task 1: Update the app-level About interaction tests

**Files:**
- Modify: `/Users/ayushns01/Desktop/Repositories/Cross-chain-bridge/web/src/app/App.test.tsx`
- Test: `/Users/ayushns01/Desktop/Repositories/Cross-chain-bridge/web/src/app/App.test.tsx`

- [ ] **Step 1: Write the failing test expectations for stage-attached explainers**

Add assertions that the About page no longer relies on the separate `Stage inspector` block and that clicking a stage reveals layered content directly from the active stage node.

- [ ] **Step 2: Run test to verify it fails**

Run: `npm test -- src/app/App.test.tsx`
Expected: FAIL because the current About page still renders the detached inspector model.

- [ ] **Step 3: Keep only the minimal expectations needed for the new interaction**

Make the test assert:
- About opens from the AegisLink menu
- the clicked stage becomes the active explainer
- layered copy such as `What’s happening now`, `Inside AegisLink`, and `Why this matters` is visible for the selected stage

- [ ] **Step 4: Re-run the focused test and confirm the failure remains intentional**

Run: `npm test -- src/app/App.test.tsx`
Expected: FAIL on the new layered explainer assertions, not on unrelated rendering issues.

### Task 2: Refactor the About content model for direct stage expansion

**Files:**
- Modify: `/Users/ayushns01/Desktop/Repositories/Cross-chain-bridge/web/src/features/about/about-content.ts`
- Modify: `/Users/ayushns01/Desktop/Repositories/Cross-chain-bridge/web/src/features/about/AboutSection.tsx`
- Test: `/Users/ayushns01/Desktop/Repositories/Cross-chain-bridge/web/src/app/App.test.tsx`

- [ ] **Step 1: Extend the stage content shape to support layered explainer copy**

Replace the current detached-inspector fields with structured fields for:
- `nowTitle`
- `nowBody`
- `systemTitle`
- `systemBody`
- `whyTitle`
- `whyBody`
- footer metadata for compact system tags

- [ ] **Step 2: Implement the direct-expansion interaction in `AboutSection.tsx`**

Use a single active stage and a pinned stage model so:
- hover previews a stage
- click pins it open
- leaving the scene returns to the pinned stage or default stage
- only one explainer capsule is visible at a time

- [ ] **Step 3: Remove the detached inspector aside**

Render the layered explainer capsule from the active stage node instead of a separate right-side panel.

- [ ] **Step 4: Run the focused app test and confirm it passes**

Run: `npm test -- src/app/App.test.tsx`
Expected: PASS

### Task 3: Redesign the About styles around stage-attached capsules

**Files:**
- Modify: `/Users/ayushns01/Desktop/Repositories/Cross-chain-bridge/web/src/styles/global.css`
- Modify: `/Users/ayushns01/Desktop/Repositories/Cross-chain-bridge/web/src/features/about/AboutSection.tsx`
- Test: `/Users/ayushns01/Desktop/Repositories/Cross-chain-bridge/web/src/features/bridge/bridge.test.tsx`

- [ ] **Step 1: Add capsule styles for the expanded stage content**

Create styles for:
- smaller dark-blue stage nodes
- attached explainer capsules
- anchor stems/glow trails
- layered explainer sections
- compact footer tags

- [ ] **Step 2: Preserve the wormhole scene as the visual hero**

Keep the portals, tunnel, and core intact while reducing any inspector-era layout pressure.

- [ ] **Step 3: Add motion and reduced-motion handling**

Animate the capsule reveal subtly and ensure reduced-motion users still get a clear, stable layout.

- [ ] **Step 4: Run the bridge regression tests**

Run: `npm test -- src/features/bridge/bridge.test.tsx`
Expected: PASS

### Task 4: Full verification

**Files:**
- Verify only

- [ ] **Step 1: Run the combined frontend tests**

Run: `npm test -- src/app/App.test.tsx src/features/bridge/bridge.test.tsx`
Expected: PASS

- [ ] **Step 2: Run the production build**

Run: `npm run build`
Expected: PASS

- [ ] **Step 3: Run diff hygiene**

Run: `git diff --check`
Expected: no output
