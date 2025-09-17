# OpenTaco Extended Scope

Not yet an MVP, not even MLP. Will need at least one more pass to distill into smaller form.

# Prior work

[OpenTaco what’s missing in the world](https://www.notion.so/OpenTaco-what-s-missing-in-the-world-2538cc53bb5a802da746de7284961633?pvs=21)

[OpenTaco memo](https://www.notion.so/OpenTaco-memo-2468cc53bb5a803dbb4fcf3397c0a2fc?pvs=21)

[OpenTaco user calls (TAB)](https://www.notion.so/OpenTaco-user-calls-TAB-2548cc53bb5a80b3bfd7dadebcf839ff?pvs=21)

https://github.com/diggerhq/opentaco-memo

[Outputs propagation](https://docs.google.com/presentation/d/1iQQd8fzW47qVl0Qv6pRQtm4MaZqtqPPr91pepAkixQI/edit?slide=id.g2c215c4c762_0_62#slide=id.g2c215c4c762_0_62)

[the case for standalone state backend manager](https://www.reddit.com/r/Terraform/comments/1l48iyf/the_case_for_a_standalone_state_backend_manager/)

# Core opinions

See [OpenTaco memo](https://www.notion.so/OpenTaco-memo-2468cc53bb5a803dbb4fcf3397c0a2fc?pvs=21) for detailed technical positions

- Built for self-hosting *with ease*
- CLI-first UX. UI exists only for infrequent admin / configuration journeys.
- Pluggable compute; initially just GHA but designed for multiple
- Manages state in a user-provided S3 bucket
- First-class RBAC (meaning also SSO)

# Peeling the onion

This piece of software is different from your average SaaS in that *how* it works is important on all levels, and not just in terms of consequences such as reliability or performance or cost. It’s first and foremost a system design exercise. We can split various functionalities into “layers”, with each higher layer requiring the prior layer fully working. As a result we get an onion-like structure, which we need to peel first before we build anything, then build the “core”, then layer 1, and so on. At which layer it becomes user-valuable is not fully clear to me yet.

# Layer 0 “state control”

It’s a CLI and a lightweight API that is only concerned with the following:

- State (CRUD + status info)
- Locking
- Dependencies (output propagation)

CLI user can:

- Authenticate (via SSO provider - GitHub at a minimum + configurability for others)
- Create new states / “stacks” / (how do we call them?)
- Configure terraform / tofu to use OpenTofu state backend proxy
- See status of the dependency graph (whats up to date / needs plan-apply)
- Lock / unlock specific states
- Define their relationships (dependency graph)
- Import from TFC (how?)
- Pull remote state into a local file
- Plan / apply locally with a local state (OpenTaco does nothing - just allows to pull state)

Importantly, this is what is NOT in scope for Layer 0

- No “runs” of any kind - neither “remote from local” nor “ci”
- No “pr automation” of any kind. No GitHub apps, nothing like that
- No UI (beyond basic login / redirect message / SSO buttons)

# Layer 1 “remote execution”

Equivalent of [CLI-driven remote runs](https://developer.hashicorp.com/terraform/cloud-docs/run/cli) in TFC

Extending the backend to be able to start jobs with external compute (initially GHA, later also K8S / Spinnaker / other CIs. Need a clean abstraction for community contributions)

CLI user can:

- run plan / apply of *local code* on 1 state
- run plan / apply of *local code* on a subtree (needs queuing / scheduling)

# Layer 2 “AAM GitHub”

- Change detection (impacted states)
- Plan preview comment in a PR
- Apply after merge

# Layer 3 “ABM extension” (optional)

PR automation functionality similar to Atlantis / Digger / Terrateam

If we do this right it doesn’t have to be mutually exclusive. 

# Layer 4 “Drift detection”

User journeys to detect and remediate drift.

Needs much more thoughtful design than mere notifications in Slack.