# Transfer Wormhole Progress Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Transform the transfer experience from a static form plus generic progress panel into a morphing card that becomes a cinematic wormhole journey after submission.

**Architecture:** Keep `TransferPage` responsible for input collection and session submission, but replace the current progress presentation with a wormhole-centric progress component that derives visual stage state from the existing `BridgeSession` model. Reuse the AegisLink visual language from the About page without duplicating the exploratory inspector interaction.

**Tech Stack:** React, TypeScript, Vitest, Testing Library, Vite CSS

---

### Task 1: Make the transfer tests fail for the new morphing progress experience

**Files:**
- Modify: `/Users/ayushns01/Desktop/Repositories/Cross-chain-bridge/web/src/features/bridge/bridge.test.tsx`
- Test: `/Users/ayushns01/Desktop/Repositories/Cross-chain-bridge/web/src/features/bridge/bridge.test.tsx`

- [ ] **Step 1: Add expectations for the cinematic progress state**

Update the transfer progress assertions so they expect:
- the wormhole progress surface after submit
- a compact transfer manifest that still shows amount, destination, and recipient
- live stage labels such as `Verifier checks`, `Bridge accounting`, `IBC handoff`, and `Osmosis receipt`

- [ ] **Step 2: Run the focused bridge tests and confirm they fail**

Run: `npm test -- src/features/bridge/bridge.test.tsx`
Expected: FAIL because the old generic progress panel does not render the new wormhole-driven progress UI.

### Task 2: Refactor the progress component around a wormhole stage model

**Files:**
- Create: `/Users/ayushns01/Desktop/Repositories/Cross-chain-bridge/web/src/features/bridge/transfer-progress.ts`
- Modify: `/Users/ayushns01/Desktop/Repositories/Cross-chain-bridge/web/src/features/bridge/ProgressPanel.tsx`
- Modify: `/Users/ayushns01/Desktop/Repositories/Cross-chain-bridge/web/src/features/bridge/bridge-session.ts`
- Test: `/Users/ayushns01/Desktop/Repositories/Cross-chain-bridge/web/src/features/bridge/bridge.test.tsx`

- [ ] **Step 1: Introduce a derived stage mapper**

Create a small helper that maps the current `BridgeSession` status into:
- active stage id
- completed stage ids
- short live headline
- short live summary

- [ ] **Step 2: Rewrite `ProgressPanel` around the new progress layout**

Render:
- a compact manifest
- a live stage callout
- a centered wormhole visual with highlighted stages
- a proof section for source and destination transactions

- [ ] **Step 3: Keep the existing transaction proof behavior intact**

Preserve:
- Sepolia tx link
- Mintscan destination link fallback
- reset action
- poll error rendering

- [ ] **Step 4: Run the focused bridge tests and confirm they pass**

Run: `npm test -- src/features/bridge/bridge.test.tsx`
Expected: PASS

### Task 3: Restyle the transfer flow to feel like a morphing card

**Files:**
- Modify: `/Users/ayushns01/Desktop/Repositories/Cross-chain-bridge/web/src/styles/global.css`
- Modify: `/Users/ayushns01/Desktop/Repositories/Cross-chain-bridge/web/src/features/bridge/TransferPage.tsx`
- Test: `/Users/ayushns01/Desktop/Repositories/Cross-chain-bridge/web/src/app/App.test.tsx`

- [ ] **Step 1: Preserve the premium input card**

Keep the transfer input card compact and calm in its initial state.

- [ ] **Step 2: Add the transformed progress styling**

Add styles for:
- dark cinematic progress shell
- compact manifest
- live stage status
- wormhole scene
- active/completed/upcoming stage markers
- proof cards and completion state

- [ ] **Step 3: Add restrained animation and reduced-motion fallbacks**

Use subtle motion for:
- wormhole pulse
- stage emphasis
- progress card entrance

- [ ] **Step 4: Run the app regression tests**

Run: `npm test -- src/app/App.test.tsx`
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
