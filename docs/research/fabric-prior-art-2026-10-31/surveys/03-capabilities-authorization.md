# Survey 03 (verified revision): authorization, identity and trust between people, organizations, machines and agents

Date 2026-10-31. Research input for the resource-fabric design; no recommendation of a final choice is made.
Original sources read: the brief; `docs/design/resource-fabric-2026-10-31.md` (§0, §1.1-1.2, §4, §5.2-5.3, §7, §9-10); `docs/design/branching-tape-2026-10-31.md` (§0-1).

Tag legend after this pass: **[V url]** = confirmed against a primary source (paper, spec, official docs/blog) this session; **[V-sec url]** = confirmed only via a secondary or vendor source, primary text not parsed; **[CORRECTED]** = original claim was wrong or imprecise, fix given; **[UNVERIFIED]** = could not be confirmed (what was tried is stated, or "not fetched" if no attempt fit the budget); **[ADDED]** = new material not in the first draft. Original tags [K]/[U]/[M]/[R] are dropped unless a claim stays unverified, in which case it carries [UNVERIFIED]. Inference and design-option statements are marked [inference] / [design option] and are not factual claims about the literature.

---

## Verification log

**Counts (approximate; counted per cited system or load-bearing claim group):** confirmed ≈ 38; corrected ≈ 14; still unverified ≈ 32 (mostly classic papers and systems that were not fetched, listed in Part K); added ≈ 9.

**Method limits.** WebFetch could not parse PDFs (Zanzibar ATC paper, Cedar PDFs, USENIX); for those, the arXiv abstract/HTML, an AWS blog, or the Google Research page were used. Several publisher sites returned 403 or certificate errors (Firecracker NSDI page, ACM DL, Cornell mirror). Where only a search snippet or a secondary summary was available, the tag says so.

**Most consequential corrections**
1. **UCAN status.** The first draft said the spec was at v1.0.0-rc.1 and 1.0 unconfirmed. The spec repository README now labels it **Version 1.0.0** (Delegation and Invocation REQUIRED; Promise and Revocation RECOMMENDED); the repo's Releases page is empty, so a formal tagged release was not seen. [CORRECTED] https://github.com/ucan-wg/spec
2. **IETF agent-delegation drafts are not WIMSE working-group documents.** Both are individual submissions ("not endorsed by the IETF"), with `wimse` only in the filename/workgroup tag; real titles differ from those given: draft-atakora-wimse-sadp-delegation-01 is "Capability-Bound Delegation Chains with Scoped Context Disclosure for AI Agents" (Experimental, 20 Sep 2026); draft-asor-wimse-agent-delegation-chain-00 is "Verifiable Attenuated Delegation for AI Agent Chains" (27 Aug 2026; profiles OAuth JWT + RAR with SHA-256 parent links, not macaroons). [CORRECTED]
3. **seL4 citations.** "Sewell et al., Translating Capabilities to Rights (2011)" could not be found under that title. The verifiable papers are Elkaduwe, Klein, Elphinstone, "Verified Protection Model of the seL4 Microkernel" (VSTTE 2008, take-grant-derived) and Sewell et al., "seL4 Enforces Integrity" (ITP 2011; proves integrity and authority confinement). [CORRECTED]
4. **"Capability Myths Demolished"** is a 2003 Johns Hopkins technical report (SRL-2003-02), reportedly rejected by USENIX Security, not a peer-reviewed paper; secondary sources list the myths as equivalence (capabilities = ACLs), confinement, and revocation. "Cannot be reviewed" was not found as a separate myth. [CORRECTED, secondary]
5. **BeyondCorp access proxy / trust inferer** are not in the 2014 ;login: article as far as could be checked; the access proxy is the subject of "BeyondCorp Part III: The Access Proxy" (Cittadini, Spear, Beyer, Saltonstall, ;login: 2016). [CORRECTED, secondary]
6. **Cedar claims** mostly confirmed (4 validator bugs found by proofs, 21 more by DRT/PBT, 18 person-days for the validator-soundness proof, release gated on model+proofs+DRT). The "earlier Dafny model" is supported only by a conference-talk snippet, not by the paper or AWS blog. The symbolic tool name `cedar-policy-symcc` is real and uses CVC5. [V, see D3]
7. **SPIRE attestors.** The SPIRE concepts page lists AWS, Azure, GCP, Kubernetes service-account-token, join-token and existing-X.509 node attestors; **TPM was not listed there** (the draft listed it and named "PSAT"). [CORRECTED]
8. **Apple Private Cloud Compute** is no longer only a bespoke Apple-silicon design: reporting says it has been extended to third-party data centers (Google Cloud with Intel TDX and NVIDIA GPUs) while keeping Apple's attestation/transparency controls. [CORRECTED, secondary]
9. **Take-Grant authorship.** Secondary sources credit Lipton and Snyder for the linear-time safety algorithm; no source seen listed Jones. [UNVERIFIED on "Jones"]
10. **Tomašev et al. "Intelligent AI Delegation"** exists (arXiv 2602.11865, 12 Feb 2026) but the draft's description ("proposes Delegation Capability Tokens on macaroons") is only second-hand; a secondary summary calls it a framework paper with no empirical evaluation and says blog write-ups mention delegation capability tokens and zero-knowledge proofs that could not be confirmed. [UNVERIFIED on contents]

---

## Part A. Foundations

### A1. Lampson's access matrix, ACLs, capabilities
- Problem: say precisely which subject may do which operation on which object. Lampson, "Protection" (1971/74) models a matrix: rows subjects (domains), columns objects, cells rights. [UNVERIFIED: not fetched]
- Mechanism: the matrix is stored by column (ACL on the object) or by row (capability list held by the subject). Dennis and Van Horn (1966) introduced capabilities as unforgeable (object, rights) pairs. [UNVERIFIED: not fetched]
- Guarantees / failure modes: ACLs keyed to requester identity → ambient authority; capabilities make "who can touch X" a global graph question and revocation needs indirection. (Conceptual summary; consistent with Hardy's essay, see A2.)
- Mechanism vs policy: Harrison-Ruzzo-Ullman (1976) showed safety is undecidable in general under weak assumptions, and decidable in restricted cases (e.g. mono-operational systems). [V-sec https://www.cs.purdue.edu/homes/ninghui/courses/Spring05/lectures/lecture06.pdf and other course notes seen in search] Take-Grant: Lipton and Snyder's linear-time safety algorithm for the take-grant rewriting system [V-sec; "Jones, Lipton, Snyder 1976" as the author list is [UNVERIFIED], no source seen names Jones]. Shapiro and Weber, "Verifying the EROS Confinement Mechanism" (IEEE S&P 2000) [UNVERIFIED: the search for the title returned only unrelated plasma-physics results]. "Miller-Shapiro Capability-based Authority Safety" as named in the draft [UNVERIFIED: no such title found; do not cite without checking].
- Relevance: Choir's "grants decide what a capability or controller may do" (§9.5) is a sparse access matrix; Q9 asks which restricted command set (grant, attenuate, revoke, expire) keeps safety decidable. [inference]

### A2. Confused deputy (Hardy 1988)
- Verified account: a compiler at a commercial timesharing service held a license over its home directory (SYSX) to write its statistics file; a user supplied the billing file's name as the debug-output target; the compiler's own authority let the write succeed and billing data was lost. Hardy's cause: the compiler drew authority from two sources (invoker and home-files license) and had no way to express which was meant; his cure is that a capability both names and authorizes. [V https://www.cs.utexas.edu/~witchel/S25-380L/papers/hardy88confused.pdf via search summary; pages at crypto.stanford.edu and css.csail.mit.edu confirm]
- [CORRECTED] The first draft implied the incident happened on KeyKOS. The essay describes a commercial timesharing system; KeyKOS is the capability system Hardy points to as the cure (KeyKOS architecture paper cited by Hardy dates from 1985). The service's name was not confirmed here.
- Choir link (unchanged, design reading): the comms computer proposing a post to Alice's computer (§7.2) is confused-deputy-resistant; a controller grant naming verbs and scope is capability-like; a bare "A is parent of B" is an identity ACL. [inference]

### A3. Object-capability (ocap) model
- Principles from Miller, Yee, Shapiro, "Capability Myths Demolished" (2003): no ambient authority, designation = authority, unforgeable references, connectivity begets connectivity, encapsulation. Refutes myths on equivalence with ACLs, confinement (cites KeyKOS factories and EROS constructor) and revocation (forwarders/facets). [V-sec https://researchr.org/publication/MillerS03 and summaries; [CORRECTED] it is a JHU tech report SRL-2003-02, not a conference paper; the myth list is equivalence/confinement/revocation, not "review"]
- Mechanism/policy: pure mechanism; policy = the reference graph, built through attenuating wrappers, caretakers, membranes (Miller thesis, A4). Failure modes: capability leaks through any reference channel; revocation needs a caretaker built in advance; audit needs invocation logging. [inference/standard summary; [UNVERIFIED] as sourced claims]

### A4. Miller, "Robust Composition" (PhD, Johns Hopkins, 2006)
- Plan interference, POLA, E language (vats, eventual send, promises), membranes, caretakers, sealer/unsealer, powerbox, SturdyRef design (swiss number + key fingerprint). [UNVERIFIED: thesis not fetched. A scribd copy appeared in search but was not opened.]
- Choir link: three gates for native capabilities (§5.2) ≈ powerbox; "tape as audit" is what ocap systems usually lack. [inference]

---

## Part B. Capability systems

### B1. KeyKOS, EROS, CapROS
- Kernel-protected c-lists, domains, keepers, space bank, constructor with confinement check, single-level-store checkpointing; EROS (Shapiro, Smith, Farber SOSP 1999); Bomberger et al. 1992. [UNVERIFIED: none fetched; the search for the Bomberger 1992 paper returned nothing]. The constructor/"factory" confinement role is supported by the Capability Myths summary [V-sec, see A3].
- Choir relevance (design reading): leaf VMs as constructor/confinement pattern; space bank ≈ budget on a controller grant. [inference]

### B2. seL4
- Verification page confirms functional correctness ("from high-level specifications to binary code") and states the assumptions: behaviour of assembly code, boot code, machine interface, hardware, DMA. [V https://sel4.systems/Verification/] That page does not name integrity, information flow, or WCET explicitly; they are covered on linked proofs/assumptions pages not opened.
- Capability-derivation/authority model: Elkaduwe, Klein, Elphinstone, "Verified Protection Model of the seL4 Microkernel" (VSTTE 2008), a take-grant variant. [V-sec https://trustworthy.systems/publications/nictaabstracts/Elkaduwe_KE_08.abstract] Integrity and authority confinement proved in Sewell et al., "seL4 Enforces Integrity" (ITP 2011). [V-sec https://www.sel4.systems/Research/pdfs/sel4-enforces-integrity.pdf] **[CORRECTED]**: "Translating Capabilities to Rights" not found. Information-flow (confidentiality) proof: Murray et al. exists (Stanford seminar page "How to Prove your Kernel Secure: putting the 's' into seL4 -- provably" describes the first confidentiality proof for a general-purpose kernel implementation) [V-sec https://crypto.stanford.edu/seclab/sem-12-13/murray.html]; the exact title in the draft was not matched. Klein et al. SOSP 2009 [UNVERIFIED: not fetched]. Binary translation validation and WCET analysis [UNVERIFIED: not on the page fetched].
- capDL: "capability distribution language" describing kernel objects and capability placement; the translator can emit an Isabelle/HOL model, enabling questions such as "can component X ever gain access to resource Y". [V https://docs.sel4.systems/projects/capdl/] The draft's phrase "prove that distribution satisfies an information-flow policy" is a reasonable reading of that page but the specific proof tooling for IF policies was not opened. This is the closest precedent for "small proven kernel, policy = configuration" (Q9). [inference]

### B3. Capsicum (Watson, Anderson, Laurie, Kennaway, USENIX Security 2010)
- Verified: USENIX Security 2010, Best Student Paper; `cap_enter` enters capability mode (only calls on held descriptors / limited global state; descendants inherit); `cap_rights_limit` can only reduce rights; combined to create kernel-enforced sandboxes; shipped experimental in FreeBSD 9.0. [V https://man.freebsd.org/cap_enter/2 ; https://www.usenix.org/conference/usenixsecurity10/capsicum-practical-capabilities-unix]
- Casper services; uses in tcpdump/Chrome [UNVERIFIED: not fetched].
- Link: pattern for a Choir host process brokering native OS capabilities (Q7). [inference]

### B4. Fuchsia / Zircon handles
- Verified: per-process handle integers (no global meaning); rights bitmask per handle; `zx_handle_duplicate` can drop rights; `zx_handle_replace` creates an equivalent (possibly reduced) handle and invalidates the original; the page describes no revocation mechanism. [V https://fuchsia.dev/fuchsia-src/concepts/kernel/handles]
- Component-framework capability routing (offer/expose/use) and statements about coarse rights and no transitive revoke [UNVERIFIED: component docs not fetched].

### B5. E, Caja, Joe-E, SES
- E (Miller, Stiegler), Caja (Google), Joe-E (Finifter et al. 2008), SES / Hardened JavaScript / TC39 Compartments, LavaMoat, Agoric. [UNVERIFIED: not fetched]
- Link: desk surface as c-list [inference].

### B6. Spritely Goblins and OCapN
- Verified: Goblins is Spritely's distributed object environment with security model, local transactions, async programming, a process persistence/upgrade model; Guile and Racket, both v0.18.0. [V https://spritely.institute/goblins/] "Time travel" was not mentioned on that page. [UNVERIFIED]
- OCapN: CapTP core, certificate-based third-party handoffs, netlayers over Tor, IBC, I2P, libp2p; draft specifications live in the `draft-specifications` folder of the GitHub repo; monthly meetings (second Tuesday); no maturity statement. [V https://ocapn.org/] Status remains "drafts, maturity unstated."
- Failure modes (gift-certificate replay subtlety, sturdyref as bearer secret): standard criticisms [UNVERIFIED: not sourced].

### B7. Cap'n Proto RPC
- Verified: Levels 1-4: L1 object references and promise pipelining; L2 persistent capabilities (save/restore); L3 three-way interactions (Bob connects directly to Carol); L4 reference equality (joining). Implementations advertise the levels they cover. [V https://capnproto.org/rpc.html]
- "Level 3 unimplemented in the reference implementation" and "descended from E's CapTP" [UNVERIFIED: page does not state either]; the E lineage is widely stated but not confirmed this session.

---

## Part C. Tokens with attenuation

### C1. SPKI/SDSI
- Verified: RFC 2693 "SPKI Certificate Theory", Sept 1999, Experimental; authors Ellison, Frantz, Lampson, Rivest, Thomas, Ylonen; boolean delegation flag; tag intersection (AIntersect) narrows permissions; validity-period intersection; timed CRLs, timed revalidations, one-time revalidations (nonce), short-lived certs as alternative; SDSI local names chain, anchored to a key or hash. [V https://www.rfc-editor.org/rfc/rfc2693] [CORRECTED, minor] the draft cited "Rivest-Lampson 1996; Ellison et al. RFC 2693, 1999" as separate; RFC 2693 lists Lampson and Rivest among six authors. Rivest/Lampson SDSI 1996 [UNVERIFIED: not fetched].
- "Poor deployment / hard tooling" is a judgement [inference].

### C2. Macaroons
- Verified: Birgisson, Politz, Erlingsson, Taly, Vrable, Lentczner, NDSS 2014 (Google); bearer credentials with nested/chained HMACs; any holder can add first-party caveats (arbitrary assertion language); each caveat folds into a new HMAC keyed by the previous signature. [V https://www.ndss-symposium.org/ndss2014/macaroons-cookies-contextual-caveats-decentralized-authorization-cloud ; slides https://www.ndss-symposium.org/wp-content/uploads/2017/09/04_3_slides.pdf]
- Third-party caveats / discharge macaroons and "verifier needs the root key": the abstract-level pages did not explain these; widely stated and consistent with the IETF draft appendix noting HMAC bearer design [V-sec: draft-atakora appendix B lists macaroons as HMAC-bearer without public-key holder binding]. Treat third-party-caveat mechanics as [UNVERIFIED] until the PDF is read.
- Fly.io: confirmed in production. Fly's "Operationalizing Macaroons" describes service tokens, deploy tokens, orchestrator `flyd` using a third-party caveat to limit access to user secrets, per-organisation keys, verification via a stateful service (`tkdb`, Go + SQLite with LiteFS/Litestream), revocation done regularly. Community threads say user-created macaroons were not yet available at the time of those posts; current state not confirmed. [V https://fly.io/blog/operationalizing-macaroons/ and https://github.com/superfly/macaroon via search]

### C3. Biscuit
- Verified from biscuitsec.org: public-key signed tokens (Ed25519 needed by new implementations); holder can create a narrower token offline; Datalog policies supplied by the app or carried in the token; per-token revocation id invalidating that token and tokens derived from it; maintained by Couprie, Delafargue, Corbière; Apache-2.0; spec with predefined test cases. [V https://www.biscuitsec.org/]
- [CORRECTED] The draft credited Clever Cloud as origin; the site does not say so (not disproved; the maintainers' affiliation was not checked). Third-party blocks and sealing: not on the landing page [UNVERIFIED]. Spec maturity unstated.
- AIP uses Biscuit for multi-hop chains: confirmed (see Part I).

### C4. UCAN
- [CORRECTED] Spec repo README: **Version 1.0.0**; sub-specs Delegation (REQUIRED), Invocation (REQUIRED), Promise (RECOMMENDED), Revocation (RECOMMENDED); releases page empty. [V https://github.com/ucan-wg/spec] The draft's "v1.0.0-rc.1" is outdated or from a different branch/tag.
- JWT-like token with issuer/audience DIDs, command and policy, proof chain; content-addressed delegations; revocation list; token bloat; Go implementation missing revocation and promise; GitHub discussion #207 on pre-authorised invocation [UNVERIFIED this pass: the discussion thread and the Go repo were not re-opened; the original draft's snippet-based note stands as unconfirmed].

### C5. ZCAP-LD
- Verified only that the W3C CCG ZCAP spec has versions up to **v0.4.0** (plus rc/draft series, v0.3.0). [V https://w3c-ccg.github.io/zcap-spec/] Field list (controller, invocationTarget, parentCapability, caveats, proof), CG-report status, JSON-LD canonicalisation cost, revocation-list services, adopters (Digital Bazaar etc.) [UNVERIFIED: v0.4.0 page not opened].

### C6. Comparison relevant to Choir
Table retained from the first draft but now explicitly dependent on the verification state above: SPKI [V RFC 2693 for public key/tag algebra/CRL-reval]; Macaroon [V HMAC chain; verifier-needs-root-key per draft appendix, V-sec]; Biscuit [V Ed25519, Datalog, revocation ids]; UCAN [V DID/delegation/revocation specs; "policy (v1)" consistent with 1.0.0]; ZCAP-LD [UNVERIFIED cells].

| Property | SPKI | Macaroon | Biscuit | UCAN | ZCAP-LD |
|---|---|---|---|---|---|
| Crypto | public key | HMAC chain (symmetric) | Ed25519 chain | public key (DID) | LD proofs [UNVERIFIED] |
| Offline attenuation | yes (new cert) | yes | yes | yes (new delegation) | yes [UNVERIFIED] |
| Verifier needs secret | no | yes, root key | no | no | no |
| Policy language | tag algebra | app-defined caveats | Datalog | policy (v1) | caveats |
| Revocation | CRL/reval | expiry + id check | revocation ids | published revocation list (Revocation spec RECOMMENDED) | revocation service [UNVERIFIED] |
| Bearer vs holder-bound | holder key | bearer | bearer unless bound [UNVERIFIED] | audience-bound | invoker-bound [UNVERIFIED] |

- The shared pattern (signed statement carrying its own proof chain; monotonic attenuation; revocation is the hard part) and the mapping of Choir site/controller grants to SPKI/UCAN-like objects are design readings [inference].

---

## Part D. Relationship-based and policy-based authorization

### D1. Zanzibar, SpiceDB, OpenFGA
- Verified (Google Research abstract): Zanzibar stores and evaluates **trillions of access control lists**, millions of authorization requests per second; 95th-percentile latency under 10 ms and availability above 99.999% over three years of production; decisions "respect causal ordering of user actions" (external consistency). [V https://research.google/pubs/zanzibar-googles-consistent-global-authorization-system/] [CORRECTED, minor] the draft said "trillions of tuples"; the abstract says ACLs.
- Zookies and the new-enemy problem (two failure modes: ordering of ACL updates, applying an old ACL to new content); Spanner's TrueTime/snapshot reads make zookies possible. [V-sec https://authzed.com/blog/new-enemies , https://authzed.com/docs/spicedb/concepts/zanzibar; the ATC paper PDF could not be parsed here]
- SpiceDB: ZedTokens are its zookie equivalent; consistency options documented include `fully_consistent`, `at_least_as_fresh`, `at_exact_snapshot`. [V-sec https://authzed.com/docs/reference/api-consistency] `minimize_latency` [UNVERIFIED in this pass, widely documented]. SpiceDB caveats using CEL [UNVERIFIED].
- OpenFGA: CNCF membership level is **unclear** (sources said both "sandbox" and "incubating") [UNVERIFIED]; "no zookies" comparison from the AuthZed blog is a competitor's claim [V-sec, interested source].
- Failure modes (central dependency, total order of writes, expensive reverse lookup) and Choir link (tape/home as the ordered log Zanzibar bought with Spanner) are analysis [inference].

### D2. OPA/Rego
- [UNVERIFIED: not fetched]. The Cedar paper's abstract reports Cedar policies as equal or more readable than OpenFGA and Rego and performing far better. [V https://arxiv.org/abs/2403.04651]. Claims that Rego lacks termination guarantees: [UNVERIFIED].

### D3. Cedar
- Paper: Cutler et al., "Cedar: A New Language for Expressive, Fast, Safe, and Analyzable Authorization", arXiv 2403.04651, submitted 7 March 2024 (15 authors); modelled in Lean with proved properties; sound and complete logical encoding enabling precise analysis such as checking that refactored policies grant the same permissions. [V https://arxiv.org/abs/2403.04651]
- Verification-guided development: arXiv 2407.01688. Proofs found and fixed **4** bugs in the validator; DRT/PBT found **21** additional bugs; Lean validator proofs are 4,686 LOC. [V https://arxiv.org/html/2407.01688] AWS blog: the validator-soundness proof took **18 person-days**; "A new version of Cedar isn't released unless its model, proofs, and differential tests are up to date." [V https://aws.amazon.com/blogs/opensource/lean-into-verified-software-development] DRT runs daily on supported versions [V arXiv HTML].
- "Earlier Dafny model": neither the paper nor the AWS blog mentions Dafny; one search snippet from a Cedar conference talk says the model was first written in Dafny. [V-sec https://secdev.ieee.org/2025/keynote-mike/] Treat as likely but secondary.
- Symbolic analysis: `cedar-policy-symcc` crate = Cedar symbolic compiler producing SMT-LIB constraints (CVC5 1.3.1 required), with counterexamples and policy-set equivalence/always-allows checks; implemented and modelled in Lean. [V https://github.com/cedar-policy/cedar (crate listing); V-sec https://docs.rs/cedar-policy-symcc and AWS blog https://aws.amazon.com/blogs/opensource/introducing-cedar-analysis-open-source-tools-for-verifying-authorization-policies] [CORRECTED] the draft's "use of the term symcc is U": it is real. Whether the symbolic compiler itself is fully proved correct: a 2026 follow-up "SymCert" claims a verified symbolic compiler [V-sec; not opened].
- `forbid` overrides `permit`, default deny, no loops/user functions, schema validator: language-design claims consistent with the above [UNVERIFIED as quoted text; the paper's abstract does not list them].
- Link to Q9/Q10 and contrast with seL4 capDL: analysis [inference].

### D4. Other policy systems
- XACML, Casbin, Oso, Cerbos, CEL, Gatekeeper/Kyverno, Teleport [UNVERIFIED: not fetched].
- PDP/PEP split and "unremovable invariants" point: design reading [inference].

---

## Part E. Identity of machines, workloads and people

### E1. SPIFFE/SPIRE
- Verified: SPIFFE ID `spiffe://trust-domain/workload-identifier`; X.509-SVID and JWT-SVID (X.509 advised where possible because tokens can be replayed); Workload API provides credentials without the caller presenting a token; trust bundles; keys short-lived and auto-rotated. [V https://spiffe.io/docs/latest/spiffe-about/spiffe-concepts/]
- SPIRE node attestors: AWS, Azure, GCP, Kubernetes (service-account tokens), join token (pre-shared, expires after use), existing X.509 certificate; workload attestors: Unix, Kubernetes, Docker. [V https://spiffe.io/docs/latest/spire-about/spire-concepts/] **[CORRECTED]** TPM was not listed there (a TPM attestor may exist as a plugin; not confirmed), and "PSAT" naming not confirmed. Federation exists as a topic (linked examples) but mechanics not described on the pages read. Failure modes (join token = bearer secret; spoofable selectors on a compromised node; identity ≠ authorization) are analysis [inference].
- Link to NET-03 vsock endpoint identity [inference, from design doc].

### E2. mTLS, Certificate Transparency
- [UNVERIFIED: not fetched]. CT as precedent for append-only Merkle-log auditing: standard but unsourced here.

### E3. WebAuthn/passkeys
- [UNVERIFIED: W3C WebAuthn not fetched]. Synced-passkey trade-offs and attestation caveats: unsourced here.
- Token binding: RFC 8471 "The Token Binding Protocol Version 1.0", Proposed Standard, Oct 2018. [V https://www.rfc-editor.org/rfc/rfc8471] (Its deployment fate, e.g. removal from browsers, was not checked.) 
- DBSC: Google announced Device Bound Session Credentials public availability for Windows on Chrome 146 (April 2026), macOS (Secure Enclave) to follow; developed in the W3C WebAppSec WG as a Working Draft, not a Recommendation; Microsoft collaborated. [V-sec https://blog.google/security/protecting-cookies-with-device-bound-session-credentials/ ; https://helpnetsecurity.com/2026/04/10/google-chrome-device-bound-session-credentials] [CORRECTED] the draft said "status [K/U]".
- [ADDED] OAuth DPoP, RFC 9449 (Sept 2023, Proposed Standard): application-level sender-constraining of access/refresh tokens by proof-of-possession of a private key; server nonces; non-extractable keys make exfiltrated tokens alone unusable. [V https://datatracker.ietf.org/doc/rfc9449/] Relevant as a standards-track way to bind bearer tokens to a device key.

### E4. BeyondCorp / zero trust
- Verified: Ward and Beyer, "BeyondCorp: A New Approach to Enterprise Security", ;login: vol. 39 no. 6 (Dec 2014), pp. 6-11; Google moves corporate apps to the public internet and drops the privileged intranet; introduces the notion of a managed device (procured and actively managed). [V https://research.google/pubs/beyondcorp-a-new-approach-to-enterprise-security/ ; https://www.usenix.org/publications/login/dec14/ward]
- [CORRECTED, secondary] "access proxy" is detailed in the 2016 Part III article; the draft's attribution of "trust inferer and access-control engine" to the 2014 piece was not confirmed.
- NIST SP 800-207 "Zero Trust Architecture", Aug 2020, authors Rose, Borchert, Mitchell, Connelly. [V https://csrc.nist.gov/pubs/sp/800/207/final]
- Choir "trust class" ↔ device trust tier, and "BeyondCorp doesn't address devices hosting others' workloads": analysis [inference].

---

## Part F. Attestation, TEEs and "can I trust a workstation with someone else's data?"

### F1. TPM, measured boot, remote attestation
- RFC 9334 (RATS architecture, Jan 2023, Informational): Attester produces Evidence; Verifier appraises it against policies and issues Attestation Results to the Relying Party; further roles (Endorser, Reference Value Provider, Relying Party Owner, Verifier Owner). [V https://www.rfc-editor.org/rfc/rfc9334] Keylime is a TPM-based remote attestation solution for Linux, with a Linux Foundation write-up mapping it to RATS. [V-sec https://confidentialcomputing.io/?p=3688]
- TPM PCR/quote/EK-AK mechanics, TOCTOU, bus sniffing of discrete TPMs, and the owner-as-attacker note: [UNVERIFIED: not fetched; standard background].

### F2. Secure Enclave / platform key stores
- [UNVERIFIED: not fetched] (Apple, Windows, Android key attestation).

### F3. Confidential VMs
- **TEE.fail** (Georgia Tech, Purdue, Synkhronix; Oct 2025): physical DDR5 interposer, **under $1,000**, off-the-shelf equipment; AES-XTS deterministic encryption is the exploited weakness; extracts keys including attestation keys from Intel SGX and TDX, and AMD SEV-SNP including with Ciphertext Hiding; also NVIDIA GPU confidential computing; AMD plans no mitigation (physical attacks out of scope), Intel says its position that physical attacks are out of scope is unchanged; no evidence of in-the-wild use. [V https://thehackernews.com/2025/10/new-teefail-side-channel-attack.html] [UNVERIFIED] the draft's "on a machine where the attacker has root": not stated in the article read.
- **Battering RAM** (KU Leuven + Univ. of Birmingham, DDR4 interposer under $50: plaintext access to SGX enclave memory and falsified SEV-SNP attestation; can be enabled after boot) and **WireTap** (Georgia Tech + Purdue, passive DDR4 interposer, SGX ECDSA key recovery). Intel/AMD/Arm: physical attacks out of scope; the two attacks depend on DDR4 and deterministic encryption, reported as not directly affecting DDR5 or TDX (hence TEE.fail). [V-sec https://thehackernews.com/2025/10/50-battering-ram-attack-breaks-intel.html ; https://www.keysight.com/blogs/en/tech/nwvs/2025/10/22/security-highlight-dram-interposer-attacks-on-confidential-computing] [CORRECTED] the draft's "detail U": now confirmed at the secondary level.
- Apple Private Cloud Compute: stateless computation, enforceable guarantees, non-targetability, no privileged runtime access, verifiable transparency (published binaries and transparency log; devices route only to attested, approved software). [V-sec https://www.edgeless.systems/blog/apple-private-cloud-compute-core-concepts-and-an-open-alternative and others] [CORRECTED, secondary] reported extension to third-party data centers (Google Cloud with Intel TDX/NVIDIA GPUs). "Apple silicon exposes no guest CVM to developers": [UNVERIFIED].
- Hardware coverage on consumer workstations (no SEV-SNP/TDX on typical consumer parts) and AMD/Intel mechanism details (SNP RMP, TDX): [UNVERIFIED: not fetched].
- Trust-class axis (a/b/c) and the reading of the §1.2 default: analysis [inference].

### F4. Alternatives to hardware trust
- [UNVERIFIED: not fetched; HE/MPC/ORAM impracticality for general agent workloads is a standard judgement].

---

## Part G. Isolation of tenants

### G1. Firecracker, Cloud Hypervisor, gVisor, Kata
- Firecracker (Agache et al., NSDI 2020): lightweight VMM for serverless; AWS/project statements: user-space code starts in as little as **125 ms**, creation rate up to 150 microVMs/s/host, under 5 MiB memory overhead, minimal device model, companion "jailer" as a second line of defence. [V-sec https://aws.amazon.com/blogs/opensource/firecracker-open-source-secure-fast-microvm-serverless/ ; NSDI page https://www.usenix.org/conference/nsdi20/presentation/agache returned 403, paper PDF not read] The 125 ms figure is a vendor best case measured on i3.metal, not a guest-boot guarantee. Rust, KVM, virtio/vsock, seccomp/cgroups/namespaces details and Lambda/Fargate use: [UNVERIFIED this pass].
- gVisor, Kata, Cloud Hypervisor, SMT/side-channel statements: [UNVERIFIED: not fetched].
- Link: isolation protects host from guest, not guest from host; implication for placing A's VM on B's machine: analysis [inference].

---

## Part H. Revocation strategies
Cross-cutting analysis; items sourced where possible, otherwise [inference]/[UNVERIFIED] as background.
1. Expiry / short-lived credentials: SPIFFE short-lived rotating SVIDs [V, E1]; SPKI short-lived certs as alternative to online checks [V, RFC 2693]; macaroons with time caveat [UNVERIFIED]; UCAN `exp` [UNVERIFIED].
2. Revocation lists / OCSP / CRL / UCAN Revocation (RECOMMENDED spec) [V] / Biscuit revocation ids [V]; CRLSet-style push [UNVERIFIED].
3. Indirection / caretaker (ocap): forwarders and revoker [V-sec, Capability Myths summary].
4. Derivation-tree revoke (seL4, KeyKOS): [UNVERIFIED this pass].
5. Epoch / fencing tokens (Kleppmann on fencing; Chubby sequencers): [UNVERIFIED: not fetched here; see survey 01].
6. Key rotation / lazy revocation (Tahoe-LAFS): [UNVERIFIED].
7. Zanzibar-style check-time with freshness token [V-sec, D1].
- No revocation recalls disclosed data; staleness window equals last refresh; fail-closed vs fail-open as a per-grant-class parameter: analysis [inference/design option].

---

## Part I. AI agents and delegation (emerging literature)
- **Tomašev, Franklin, Osindero (Google DeepMind), "Intelligent AI Delegation"**, arXiv 2602.11865, 12 Feb 2026, cs.AI: delegation includes transfer of authority, responsibility, accountability, role/boundary specification, intent clarity, trust mechanisms; three scenarios (human→AI, AI→AI, AI→human). [V-sec https://www.arxiv.org/abs/2602.11865 via search] The draft's "proposes Delegation Capability Tokens on macaroons" is **[UNVERIFIED]** (blog write-ups mention delegation capability tokens and ZK proofs; the paper was not opened).
- **Prakash, "AIP: Agent Identity Protocol for Verifiable Delegation Across MCP and A2A"**, arXiv 2603.24775, single author Sunil Prakash, submitted 25 March 2026 (cs.CR): Invocation-Bound Capability Tokens (IBCTs) in two formats: compact signed JWT (single hop) and chained Biscuit token with Datalog (multi-hop); Python and Rust implementations; reported ≈0.22 ms overhead in an MCP-over-HTTP deployment, 2.35 ms added latency in a multi-agent test, and 600 attack attempts all rejected (self-reported); claims no prior protocol combined the required properties. [V https://arxiv.org/abs/2603.24775] The abstract does not mention UCAN, macaroons or the Tomašev paper, so the earlier note that the AIP paper criticises UCAN token bloat or cites Tomašev is **[UNVERIFIED]** (full text not read).
- **CAPMAS**, "Capability-Based Delegation of Privileges in Multi-Agent Systems", arXiv 2609.06500, Veski, Guerraoui, Froelicher, 6 Sept 2026 (cs.MA, cross-listed cs.CR): contrastive-learning pipeline mapping natural-language queries to bounded privilege sets, plus macaroon-based tokens for offline tamper-evident delegation where privileges only shrink; claims ≈30× faster delegation than RFC 8693 Token Exchange, >90% retrieval of the correct privilege bundle within 17 ms on schemas with >3,100 endpoints, and 99.5% reduction of unnecessary privileges (self-reported). [V https://arxiv.org/abs/2609.06500] [CORRECTED] the draft said "mentions Biscuit"; the abstract does not.
- **IETF individual drafts** (neither adopted by a WG; both "not endorsed by the IETF"): draft-atakora-wimse-sadp-delegation-01, "Capability-Bound Delegation Chains with Scoped Context Disclosure for AI Agents", Hamdy Atakora (Imara Labs), 20 Sep 2026, Experimental: chains of signed capability grants bound to end-to-end encrypted task capsules, caveats following Macaroons, hash-linked audit record; its appendix lists Macaroons as lacking public-key holder binding. [V https://datatracker.ietf.org/doc/draft-atakora-wimse-sadp-delegation/] draft-asor-wimse-agent-delegation-chain-00, "Verifiable Attenuated Delegation for AI Agent Chains", Rafael Asor (Attenu), 27 Aug 2026: OAuth JWT access tokens + Rich Authorization Requests, SHA-256 parent links, offline verification of monotonic attenuation; notes RFC 8693 records prior actors but does not enforce limits across chains of depth ≥2; "attenuation does not itself invalidate the parent". [V https://datatracker.ietf.org/doc/html/draft-asor-wimse-agent-delegation-chain-00] [CORRECTED] draft's "in the WIMSE working group" and drafts' titles.
- **OAuth family:** RFC 8693 Token Exchange (Proposed Standard, 2020) with the `act` claim for delegation, nested `act` as history, with prior actors informational only for access control [V https://www.rfc-editor.org/rfc/rfc8693]; RFC 9396 Rich Authorization Requests (2023) [V https://www.rfc-editor.org/rfc/rfc9396]; RFC 9635 "Grant Negotiation and Authorization Protocol (GNAP)" [V title; date not seen on page, publication date not asserted] https://www.rfc-editor.org/rfc/rfc9635.
- **MCP:** the Security Best Practices document describes the confused-deputy attack against MCP proxy servers (static client ID + dynamic client registration + third-party consent cookie → authorization code theft) and forbids token passthrough, requiring per-client consent. [V https://modelcontextprotocol.io/specification/draft/basic/security_best_practices] That document points to OAuth 2.1 guidance; the authorization spec itself was not opened [UNVERIFIED that it is "OAuth 2.1-based"].
- **CaMeL:** Debenedetti et al., "Defeating Prompt Injections by Design", arXiv 2503.18813 (title confirmed): uses capabilities as metadata attached to values to prevent exfiltration via unauthorised data flows; policies checked at interpreter execution time; reports solving 77% of AgentDojo tasks with provable security in v2 (67% in an earlier version) vs 84% undefended. [V-sec https://huggingface.co/papers/2503.18813 and mirrors] The "privileged planner / quarantined LLM" split is [UNVERIFIED in snippets] but is consistent with the paper's known design.
- **Willison's "lethal trifecta"** (private data + untrusted content + external communication), June 2025 [V-sec https://aiweekly.co/alerts/willison-names-lethal-trifecta-behind-ai-agent-data-leaks]; [ADDED] Meta's "Agents Rule of Two" (late Oct 2025), at most two of three per session, human approval if all three [V-sec, same search; primary not read].
- Takeaway (neutral): literature converges on macaroon/Biscuit-style attenuation (or OAuth-profile chains) plus enforcement outside the model; no production-proven standard; the sources are self-reported preprints and individual drafts. [inference]

---

## Part J. Mapping to the brief's questions
Analysis section; unchanged in substance from the first draft, with factual dependencies updated.

### J1. Q3: access control between persons, organizations, sites and computers
(Design-doc facts unchanged: default deny; personal workstation admits only enroller's work; cross-person placement requires a grant naming site, other person's computer and work class, signed by the site's principal and allowed by org policy; org policy alone cannot place A on B's machine; controller grants as owner acts on the controlled tape, cannot exceed granter's authority.)
Design dimensions [inference throughout]:
1. **Where grant truth lives:** (a) signed offline-checked statements (SPKI/UCAN/Biscuit/ZCAP) → revocation window; (b) looked-up tuples (Zanzibar) → freshness and auditability, needs reachable home, new-enemy problem handled by a zookie-like token (tape offset or lease epoch is a natural zookie); (c) hybrid: signed grants as proof, epoch/revocation head at the home checked at admission when reachable.
2. **Two-signature rule:** conjunctive attenuation; org policy can forbid but never grant at a site it does not own (cf. Cedar forbid-overrides); property to prove: no decision path where the org alone yields permit on a personal site.
3. **Identity of principals:** people: passkeys with method lattice; sites: host key + attestation; computers: ComputerID + event-chain keys; orgs: key set with roles (missing per §7.3).
4. **Delegation chains:** person → computer policy → controller grant → leaf lease token; token vs tape-record trade-off.
5. **Audit:** tape gives tamper evidence; refusal records by a site that cannot write the placed tape need a site-local hash-chained log published later; transparency-log analogies (CT, Trillian) [UNVERIFIED as sources]; non-equivocation via signed heads and witnesses.
6. **Review:** with grant/attenuate/revoke/expire and an acyclic controller graph, reachability is polynomial (Take-Grant flavour) [inference from the Lipton-Snyder result, V-sec].

### J2. The owner's constraint: org will not let employee A's VM run on employee B's workstation
Options O1-O8 are unchanged analysis (per-person site; org-owned pooled sites; bilateral grants; work-class-limited placement; confidential-computing-backed placement; key-withholding; org policy as veto only; tiered lattice with invariants I1-I3). Factual updates: O5 is further weakened by the sourced facts that TEE.fail (<$1,000, DDR5, SGX/TDX/SEV-SNP) and Battering RAM / WireTap (DDR4) defeat TEEs against physical interposers and vendors treat physical attacks as out of scope [V, F3]. Isolation (Firecracker etc.) protects host from guest, not guest from host [inference].
Considerations for all: a site grant means "B agrees to host and A agrees to be hosted", not technical confidentiality; the audit trail deters rather than prevents.

### J3. Q6: one computer controlling others without writing their state
Mappings (membrane/caretaker; broken promises as in E/Cap'n Proto promises; confused-deputy avoidance via invoker-key audience; Take-Grant over the grant DAG; K8s controller analogy) are analysis [inference]. Open trade-off: grant checked at the home only vs also embedded in each proposal as a signed token.

### J4. Q8: organization lifecycle
Models (SCIM, IdP lifecycle, Workspace admin) [UNVERIFIED: not fetched]; Choir stance and dimensions (a)-(h) unchanged [inference].

### J5. Q7: mediating native OS capabilities
Literature mappings (Capsicum [V], powerbox, TCC, Android permissions, Flatpak portals, Fuchsia routing) as before; TCC/Android/Flatpak [UNVERIFIED]. Platform direction: Apple announced on 2 Oct 2026 that it will add "additional controls" to macOS Full Disk Access because of AI agents, with users able to grant it only through very explicit action; no timeline or macOS version given; Apple's wording targets apps granted access rather than agents as a category. [V-sec https://macrumors.com/2026/10/02/apple-announces-macos-full-disk-access-changes ; https://thenextweb.com/news/apple-full-disk-access-ai-agents-mac ; https://blakecrosley.com/blog/macos-full-disk-access-ai-agents] [CORRECTED] the draft said "not independently re-fetched"; now confirmed at secondary level; Apple's developer-news primary page was not opened.

### J6. Q9: mechanism vs policy; verifying a configuration space
| System | What is small and proven | What is configuration | What is verified about configurations |
|---|---|---|---|
| seL4 + capDL | kernel (functional correctness [V]; integrity/authority confinement [V-sec]; confidentiality [V-sec]) | initial capability distribution | capDL → Isabelle/HOL model so tools can answer reachability-of-authority questions [V] |
| Cedar | evaluator and validator (Lean model, proofs, DRT [V]) | policy sets and schemas | symbolic compiler (SMT, CVC5): equivalence, always-allow/deny, counterexamples [V/V-sec] |
| Zanzibar-class | none formally | namespace configs | schema checks and tests [UNVERIFIED] |
| TLA+/Alloy at AWS | design-level protocols | parameters | model checking; Newcombe et al., CACM 58(4), April 2015, TLA+ used on 10 large systems at time of writing [V-sec https://cacm.acm.org/ , Lamport's abridged version https://lamport.azurewebsites.net/tla/amazon-excerpt.html] |
| P language, deterministic simulation (FoundationDB, TigerBeetle), Jepsen | protocols | fault schedules | simulation / testing [UNVERIFIED: not fetched] |
| Take-Grant / authority analysis | safety theorem | the graph | decide whether a right can leak [V-sec] |
Pattern (small invariant set; analyzable configuration language; checker; implementation-vs-model testing) and risks (proofs cover the model; seL4 assumptions [V]; Cedar entity data; specification risk; state explosion; cross-site interleavings) are analysis [inference].
Q10 pointer: a monotonicity certificate (new config ≤ envelope) as in Cedar's policy comparison; envelope changes only by an owner act. Not a recommendation. [design option]

---

## Part K. Open items and unverified claims
Still **[UNVERIFIED]** after this pass (not fetched or not confirmable): Lampson 1971, Dennis-Van Horn 1966; Take-Grant authorship (Jones); Shapiro-Weber EROS confinement paper; "Miller-Shapiro Capability-based Authority Safety"; Miller thesis details; KeyKOS/EROS/CapROS papers and Bomberger 1992; Klein SOSP 2009; seL4 WCET/binary validation on the proofs pages; Fuchsia component-routing docs; E/Caja/Joe-E/SES; Goblins "time travel"; Cap'n Proto "Level 3 unimplemented"; macaroon third-party-caveat mechanics; Biscuit third-party blocks and sealing, Clever Cloud origin; UCAN Go-library status and GH discussion #207; ZCAP-LD fields/status/revocation/adopters; SpiceDB `minimize_latency` and CEL caveats; OpenFGA CNCF level; OPA/Rego, XACML, Casbin, Oso, Cerbos; Cedar "earlier Dafny" (secondary only) and `forbid`/default-deny proof statements; mTLS/CT; WebAuthn/passkeys; TPM mechanics; Secure Enclave and Android/Windows attestation; SEV-SNP/TDX/CCA details; Apple-silicon CVM claim; HE/MPC/ORAM; Firecracker/gVisor/Kata internals; Tomašev paper contents; AIP and CAPMAS full texts; MCP authorization spec being OAuth 2.1-based; CaMeL planner/quarantined-LLM split; SCIM and offboarding practice; Newcombe paper claims beyond the abstract; TCC/Android/Flatpak portals; P language/FoundationDB/TigerBeetle/Jepsen.
Not covered in depth: Kerberos/OIDC federation, DID Core (W3C VC Data Model 2.0 only seen as Candidate Recommendation Drafts dated 2024; Recommendation status not confirmed), threshold signatures for org keys, Intel TDX Connect / confidential GPUs, Tailscale/ZeroTier-style grant models, Macaroon-adjacent systems (Google's internal ACL practice), Cloudflare Workers capability bindings, Hyperledger/AnonCreds.

---

## Added prior art (this pass)
- **[ADDED] WASI 0.2 / Component Model:** all access to external resources is via capabilities, mainly handles defined in the component-model type system; handles are unforgeable and can only be obtained by being passed from another instance; no ambient authority (no global namespaces at runtime or global functions at link time); P2 components declare each capability as a separate WIT import, so the host instantiates with only granted imports and the interface contract is statically inspectable; async arrives with WASI 0.3. [V-sec https://wasi.dev/security and https://github.com/WebAssembly/WASI] Candidate precedent for the "manifest of the capabilities it carries" idea (§5.2).
- **[ADDED] RFC 9449 DPoP** (E3 above).
- **[ADDED] Cedar SymCC / SymCert / AutoCedar:** symbolic compiler crate (version 0.1.3, Nov 2025) [V-sec https://docs.rs/cedar-policy-symcc]; SymCert, a Lean framework for verified SMT-based Cedar analyses (2026) [V-sec, not opened]; AutoCedar (arXiv 2607.03656), an agentic framework for verifier-guided access-control policy synthesis, which uses the compiler to compare permit sets via set difference [V-sec https://arxiv.org/pdf/2607.03656, snippet only]. Relevant to Q10: an LLM proposes policy and a verifier gates it.
- **[ADDED] Meta "Agents Rule of Two"** (Part I).
- **[ADDED] DBSC** status (E3).
- **[ADDED] Apple PCC** details and third-party data-center extension (F3).
- **[ADDED] Hardy essay primary text** details (A2).
- **[ADDED] Elkaduwe-Klein-Elphinstone and "seL4 Enforces Integrity"** (B2).
- **[ADDED] RFC 9334 roles; Keylime↔RATS mapping** (F1).

---

## Primary sources (links opened or surfaced this session)
Opened and confirmed [V]:
- Cedar: https://arxiv.org/abs/2403.04651 ; https://arxiv.org/html/2407.01688 ; https://aws.amazon.com/blogs/opensource/lean-into-verified-software-development ; https://github.com/cedar-policy/cedar
- Zanzibar abstract: https://research.google/pubs/zanzibar-googles-consistent-global-authorization-system/
- AIP: https://arxiv.org/abs/2603.24775 ; CAPMAS: https://arxiv.org/abs/2609.06500 ; Tomašev et al.: https://www.arxiv.org/abs/2602.11865 (search summary)
- UCAN: https://github.com/ucan-wg/spec
- IETF drafts: https://datatracker.ietf.org/doc/draft-atakora-wimse-sadp-delegation/ ; https://datatracker.ietf.org/doc/html/draft-asor-wimse-agent-delegation-chain-00
- RFCs: https://www.rfc-editor.org/rfc/rfc2693 , rfc8693 , rfc8471 , rfc9396 , rfc9635 , rfc9334 ; https://datatracker.ietf.org/doc/rfc9449/
- NIST SP 800-207: https://csrc.nist.gov/pubs/sp/800/207/final
- TEE.fail: https://thehackernews.com/2025/10/new-teefail-side-channel-attack.html
- seL4: https://sel4.systems/Verification/ ; https://docs.sel4.systems/projects/capdl/
- Capsicum: https://man.freebsd.org/cap_enter/2
- Fuchsia handles: https://fuchsia.dev/fuchsia-src/concepts/kernel/handles
- Cap'n Proto RPC: https://capnproto.org/rpc.html
- OCapN: https://ocapn.org/ ; Goblins: https://spritely.institute/goblins/
- Biscuit: https://www.biscuitsec.org/ ; ZCAP-LD index: https://w3c-ccg.github.io/zcap-spec/
- SPIFFE: https://spiffe.io/docs/latest/spiffe-about/spiffe-concepts/ ; SPIRE: https://spiffe.io/docs/latest/spire-about/spire-concepts/
- MCP security best practices: https://modelcontextprotocol.io/specification/draft/basic/security_best_practices
Surfaced via search only [V-sec]: NDSS macaroons page; Fly.io "Operationalizing Macaroons"; AuthZed docs and blogs; Hardy essay copies; Capability Myths summaries (researchr); Elkaduwe et al. and Sewell et al. at trustworthy.systems/sel4.systems; Keysight/Hacker News on Battering RAM/WireTap; Apple PCC coverage; DBSC coverage; MacRumors and others on Apple FDA; BeyondCorp pages at research.google and usenix.org; CaMeL mirrors; Willison/lethal trifecta coverage; docs.rs cedar-policy-symcc; wasi.dev.
Classic references still to be checked by a later pass are those listed under Part K.
