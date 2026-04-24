export type BridgeStage = {
  id: "deposit" | "verify" | "credit" | "route" | "deliver";
  title: string;
  eyebrow: string;
  summary: string;
  nowTitle: string;
  nowBody: string;
  systemTitle: string;
  systemBody: string;
  whyTitle: string;
  whyBody: string;
  footerTags: string[];
  accent: "pink" | "violet" | "gold";
};

export const bridgeStages: BridgeStage[] = [
  {
    id: "deposit",
    title: "Deposit signed",
    eyebrow: "01 · Sepolia",
    summary: "Source transaction broadcast",
    nowTitle: "What's happening now",
    nowBody:
      "The connected Sepolia wallet submits the bridge transaction and starts the cross-chain session.",
    systemTitle: "Inside AegisLink",
    systemBody:
      "The bridge session begins from a real source transaction identity that the rest of the flow can track and verify.",
    whyTitle: "Why this matters",
    whyBody:
      "The transfer now has a concrete on-chain origin instead of a vague request, so every later step can bind back to it.",
    footerTags: ["Sepolia", "Source tx", "Session start"],
    accent: "pink",
  },
  {
    id: "verify",
    title: "Verifier checks",
    eyebrow: "02 · Verify",
    summary: "Evidence and replay safety",
    nowTitle: "What's happening now",
    nowBody:
      "AegisLink validates the deposit evidence, checks replay safety, and confirms the transfer matches bridge policy.",
    systemTitle: "Inside AegisLink",
    systemBody:
      "Signer policy, verifier logic, and replay protection decide whether the deposit is acceptable enough to enter bridge accounting.",
    whyTitle: "Why this matters",
    whyBody:
      "Only valid deposits can advance, which keeps the bridge from crediting duplicated or malformed source activity.",
    footerTags: ["Verification", "Signer policy", "Replay defense"],
    accent: "violet",
  },
  {
    id: "credit",
    title: "Bridge accounting",
    eyebrow: "03 · Zone",
    summary: "Value enters the bridge zone",
    nowTitle: "What's happening now",
    nowBody:
      "Once verification passes, the bridged value is credited inside the AegisLink bridge zone before outbound routing begins.",
    systemTitle: "Inside AegisLink",
    systemBody:
      "Source confirmation becomes internal bridge-owned state, which is what allows AegisLink to control the destination leg.",
    whyTitle: "Why this matters",
    whyBody:
      "This separates source confirmation from destination delivery and makes it clear that the bridge now owns the next action.",
    footerTags: ["Bridge zone", "Credit state", "Outbound ready"],
    accent: "violet",
  },
  {
    id: "route",
    title: "IBC handoff",
    eyebrow: "04 · IBC",
    summary: "Live destination delivery begins",
    nowTitle: "What's happening now",
    nowBody:
      "AegisLink initiates the outbound route toward Osmosis and turns the transfer into a real cross-chain delivery leg.",
    systemTitle: "Inside AegisLink",
    systemBody:
      "Route, timeout policy, and packet state are created so the delivery leaves the bridge zone with concrete destination instructions.",
    whyTitle: "Why this matters",
    whyBody:
      "This is the moment where the system turns verification into cross-chain delivery instead of stopping at internal accounting.",
    footerTags: ["IBC", "Timeout policy", "Packet movement"],
    accent: "gold",
  },
  {
    id: "deliver",
    title: "Osmosis receipt",
    eyebrow: "05 · Receipt",
    summary: "Destination settlement and proof",
    nowTitle: "What's happening now",
    nowBody:
      "The transfer lands on Osmosis and resolves into the final destination transaction and receipt.",
    systemTitle: "Inside AegisLink",
    systemBody:
      "The bridge session is matched back to the destination transaction so the frontend can show the final receipt with confidence.",
    whyTitle: "Why this matters",
    whyBody:
      "The user can inspect real settlement directly instead of trusting a generic completed state or an internal-only status.",
    footerTags: ["Osmosis", "Destination tx", "Settlement proof"],
    accent: "gold",
  },
];

export const aboutDocs = [
  {
    title: "System Architecture",
    summary:
      "How Ethereum, AegisLink, relayers, and the destination chain fit together.",
    href: "https://github.com/ayushns01/AegisLink/blob/master/docs/architecture/01-system-architecture.md",
  },
  {
    title: "Security Model",
    summary:
      "Verifier assumptions, replay protection, bridge policy, and trust boundaries.",
    href: "https://github.com/ayushns01/AegisLink/blob/master/docs/architecture/02-security-and-trust-model.md",
  },
  {
    title: "Demo Walkthrough",
    summary:
      "The fastest way to explain a real end-to-end bridge run to reviewers or recruiters.",
    href: "https://github.com/ayushns01/AegisLink/blob/master/docs/demo-walkthrough.md",
  },
  {
    title: "Public Bridge Ops",
    summary:
      "The one-command backend flow and how the live public path is bootstrapped.",
    href: "https://github.com/ayushns01/AegisLink/blob/master/docs/runbooks/public-bridge-ops.md",
  },
];
