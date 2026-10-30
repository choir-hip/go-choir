# Two decisions for today

Status 2026-10-30: the two owner decisions here are from 2026-10-26 and are probably answered since (not confirmed in this pass); see [ACTIVE](../ACTIVE.md).

I wrote this on Monday morning, October twenty-sixth, after the overnight letter. It walks through the two open decisions that are yours to make, with the options and what each one leads to. The overnight letter named four decisions. You settled two of them this morning: d@e's engineer now runs on DeepSeek, and related stories is switched on.

Both open decisions have a safe default, and I recommend one option for each, but each one changes how the product behaves for real users.

## Decision one: which owner acts should ask you to prove yourself again

Here is the situation. An owner act is anything only the owner may do. That includes approving a self-development change, applying it, restoring an earlier state, rebuilding a computer from its tape, and publishing a release. Today, any signed-in browser session can do every one of them. The session cookie lasts thirty days. Your passkey is checked only when you sign in, never again.

That matters because whoever holds the cookie holds every owner act. Someone who steals it, or a script that runs on your page, could approve code onto your computer. A known problem record says a guest-served desktop runs on your own origin without a content security policy, so the second path is not imaginary. This is the one open security item marked blocking in the residual register.

The first option is step-up for the acts that change code or cannot be undone. That means approve, apply, restore, rollback, rebuild, publishing a release, and minting an API key that can perform owner acts. Each would ask for your passkey again, which on a phone or a Mac is a touch. Everyday acts, such as refreshing a computer, would not ask. Agents that work for you hold API keys, so they are not slowed down. The key is minted once, behind the step-up. This is my recommendation.

The second option is step-up for every owner act. It is the safest, but you would touch your passkey for routine things, and people learn to approve prompts without reading them.

The third option adds a short grace period to either of the first two: after one step-up, further acts go through for about fifteen minutes. A batch of approvals then costs one touch. I would add this to the first option.

The fourth option is to wait until launch. The risk today covers only you and the test accounts. The cost is that the hole opens the day a real user signs up, and the fix is red-class work on the sign-in path that should not be rushed at launch.

Either way, the build is a change to the sign-in path, reviewed by two seats, about a day of work. I would start it today as part of finishing what we started.

## Decision two: what happens when a computer's own change meets a platform update

Here is the situation. When a computer applies a change it made to itself, it goes idle. The host then updates idle computers to the newest platform build. If the computer's change is not yet part of that build, the computer sees its own change as foreign. Every further self-development request is refused until the two are reconciled. Last night this cost one Reader round. This morning I avoided it by landing the change on main before the deploy finished.

For our test computers that is a race I can manage by hand. For a real user it is permanent. Someone who changes their own Mail app and never publishes the change would have self-development switched off after every platform update. So this decision is really about every user's customizations, not about our test runs.

Your update rulings from Sunday already set the priorities. Security comes first, then availability, then rebasing the user's customizations. A computer is never held back from a security fix.

The first option is that the host refuses to update a computer whose applied change is not yet on main. It is small and quick to build. But it contradicts your rulings: a user with private customizations would stop getting updates, security updates included. At most it could be a stopgap for the two test computers, with security updates exempt. I do not recommend it.

The second option is to build the stack rebase, which was designed on Saturday and never built. After an update, the computer automatically opens one rebase round. The engineer re-applies the change on the new base, builds, tests, and freezes a candidate. If a file conflicts, only that file is named and redone. You approve the result, and self-development opens again. This matches your own words from Saturday: resubmit whatever differences remain, without starting over. It also respects the security-first rule, because the update always happens and the customization follows it. The cost is red-class work: a new kind of engineering round, a new gate rule, and a check that refuses a candidate built on an older base. Two seats review it, and it takes a few days. This is my recommendation.

The third option is to land changes on main automatically after they are applied. I do not recommend it. It breaks your ruling that changes stay private until the user publishes them.

If you choose the stack rebase, I keep landing changes before deploys by hand until it is built. A round may occasionally still be lost, as one was last night.

## Deferred, not forgotten

The seven email questions wait for tomorrow or later, as you said. They cover the plan, sequential numbers, existing accounts, sending, confirmation clicks, republishing policy, and the short address forms. They are written down in the email review.

## What I would do

Say yes to step-up for the code-changing acts, with a fifteen-minute grace period. Say yes to building the stack rebase. Both fit today's theme of finishing what we started, and both are designed. I can start either the moment you answer.

## Names and receipts

- Step-up: residual register PR-12; docs/problems/no-step-up-and-no-platform-owner-2026-10-22.md; internal/proxy/owner_acts.go lines 25 to 39 (cookie sessions pass every owner act); refresh token 720 hours (internal/auth/config.go); owner ruling R1 of 2026-10-23 covers vault acts only.
- Refresh race: residual register SD-01; docs/problems/promoted-stack-stays-base-moved-2026-10-26.md; U4 design in docs/design/computer-update-architecture-rnd-2026-10-24.md, "U4 design: rebasing an applied stack onto a new base", switch source.rebase_offer; owner update rulings, launch-first-pass ledger 19:0x on 2026-10-25.
- Email questions: docs/archive/design/email-numbered-addresses-and-autopaper-inbox-review-2026-10-26.md.
