# OpenTaco = State + RBAC

I’m now fully convinced that OpenTaco is NOT about “ci for terraform” aka “automation” at all.

The “core” of OpenTaco is about controlling who can read / write the state files, and only that.

Automation with compute that runs the jobs is a natural extension on top of the core.

It could even be that Digger the PR automation tool is complimentary to OpenTaco.

# Prior work

### Immediately preceding

- [OpenTaco user calls](https://www.notion.so/OpenTaco-user-calls-TAB-2548cc53bb5a80b3bfd7dadebcf839ff?pvs=21) (TAB) (continously updated)
- [OpenTaco extended scope](https://www.notion.so/OpenTaco-Extended-Scope-2548cc53bb5a80f4aadfed49925b3e7d?pvs=21)

### Earlier takes:

- [OpenTaco what’s missing in the world](https://www.notion.so/OpenTaco-what-s-missing-in-the-world-2538cc53bb5a802da746de7284961633?pvs=21)
- [hierarchy of TACO needs](https://docs.google.com/presentation/d/1l5XaXx2NACUNxY7zAaV8xW3OBk6i2Izd7gXgHNF5BYU/edit?usp=sharing)
- [OpenTaco memo](https://www.notion.so/OpenTaco-memo-2468cc53bb5a803dbb4fcf3397c0a2fc?pvs=21) (initial product / technical hunches)
- [opentaco-memo](https://github.com/diggerhq/opentaco-memo) readme (high level difference vs PR automation)
- [Outputs propagation](https://docs.google.com/presentation/d/1iQQd8fzW47qVl0Qv6pRQtm4MaZqtqPPr91pepAkixQI/edit?slide=id.g2c215c4c762_0_62#slide=id.g2c215c4c762_0_62) (slides that visualise the need for “state of the graph”)
- [the case for standalone state backend manager](https://www.reddit.com/r/Terraform/comments/1l48iyf/the_case_for_a_standalone_state_backend_manager/)
- [interstate-poc](https://github.com/ZIJ/interstate-poc) (prototype of a state backend proxy)

# Problems to solve in OpenTaco v0

No other things matter for v0 - just the problems listed below

- CLI-first UX - we must build a world-class TUI
- RBAC - who can do what (+ first-class SSO integration)
- State management + opinion on Workspaces (likely *none* but TBD) + State backend API
- Dependencies + “state of the graph” (outputs propagation; status API)
- Importing from TFC
- Locking / unlocking of states (manual; and possibly subtrees of the graph)
- TFVars / secrets (or integration with external like Vault / Infisical)

# Non-problems for v0

- **No runs / compute of any kind.** OpenTaco v1 will likely have some notion of runners / workers; but not the v0. Running the jobs is an “outer layer” relative to the “core” which is v0. The core is literally just State + RBAC and has immense value on its own.
- No VCS integration of any kind. This follows from “no runs”.
- No Drift
- No Policies
- No UI / Dashboard (it will likely be needed at some point, especially for Runs and mgmt; but for now let’s focus on designing a clean API + CLI UX aka TUI. Comment-ops will actually be calling the same API but in response to events

# CLI-first UX aka TUI

- CRUD state files (we should probably call them Units - TODO related to dependencies)
- Status of each Unit - needs re-applying or not based on recorded applies
- Download the state file (RBAC permitting - all operations are authenticated)
- Upload the new state file after local apply (RBAC permitting again)

# RBAC aka “who can do what” + SSO

This is really what TACO is about - *granular control of access to state files*

## Who?

- robust model of orgs / users / teams
- integrations with external systems of record (GitHub, Okta)

The main challenge here is to come up with a model that:

- is flexible enough in the enterprise setting
- is not duplicating existing standard functionality of roles/teams that likely is already deployed

## Can do what?

This is about access to Units (aka state files)

- CRUD (can the user see it? can they change it?)
- Lock/unlock
- Edit TFVars / secrets
- Edit the dependency graph
- Privileges to modify RBAC (how does it look in TUI?)

# State management + Workspaces

State file is the core (only?) entity OpenTaco is initially concerned with

Workspaces need further thought - likely a decision not to support it, but TBD.

We will likely call them Units to avoid existing terminology confusion 

There will likely be *auxilary stuff* around state, e.g. TFVars. Unit = State + XYZ.

Maybe a concept like Workspaces but Hashi moved away from it - why?

Spacelift also only has Stacks, no grouping by environment - why?

User journeys:

- CRUD states
- Import from TFC
- Download / Upload raw statefile
- State backend proxy (makes it easy to capture output updates) + lock support. It likely needs to support AWS-like auth. TODO.

We will greatly benefit from being backwards-compatible with existing state buckets convention. So that users can just deploy OpenTaco with their existing state buckets (including managed by Terragrunt - TODO study its conventions). The main benefit is ease of selfhosting. This is possible because we can trade performance on the data store because Terraform itself is slow, so that the store is unlikely to be a bottleneck (need some benchmarking to confirm).

Making S3 a “singular store of state” is also beautifully simple from a purely technical standpoint. If “serverless postgres” vendors like Neon or Xata can do it for perf-sensitive applications at scale, surely we can figure something too!

See also:

- [Bucket Only - No Database](https://www.notion.so/OpenTaco-memo-2468cc53bb5a803dbb4fcf3397c0a2fc?pvs=21)
- [interstate-poc](https://github.com/ZIJ/interstate-poc) (prototype of a state backend proxy)

# Dependencies aka “state of the graph”

This is solved by both TFC (stacks) and Spacelift (dependencies), to some extent Terragrunt. If we are behind on the core functionalities, we’ll lose straight out of the gate.

User journeys:

- Edit dependency graph (what is the optimal TUI?)
- See status of each Unit to make decisions whether to plan / apply

Earlier takes:

- [Outputs propagation](https://docs.google.com/presentation/d/1iQQd8fzW47qVl0Qv6pRQtm4MaZqtqPPr91pepAkixQI/edit?slide=id.g2c215c4c762_0_62#slide=id.g2c215c4c762_0_62) (slides that visualise the need for “state of the graph”)
- [the case for standalone state backend manager](https://www.reddit.com/r/Terraform/comments/1l48iyf/the_case_for_a_standalone_state_backend_manager/) (video posted on r/Terraform)

# Importing from TFC

If we don’t make it easy we lose straight out of the gate, it’s that simple

It may or may not be simple to build though; but important to be *super easy*

User journeys:

- Import state files
- Import *other things* (what are they? TODO. TFVars? Dependency graph?)

Demo (important to define what “done” means here):

1. Show complex TFC setup
2. Show how easy it is to import it into OpenTaco
3. Show smth that implies that user will never need to go to TFC again (except for runs)

# Locking / unlocking

Locks solve for potential conflicts of applying against the same state (outside of automatic lock obtained by Terraform). Equivalent functionality in TFC

Realisation: PR locks are a special case of general-purpose long-lived locks. Later on when we add runners and PR automation we can use the same locking API.

User journeys:

- Lock a unit
- Unlock a unit
- Maybe also: lock a subtree
- Maybe also: unlock a subtree
- Maybe also: implicit lock during `taco state pull`

# TFVars / secrets

This one is the least clear of all but deserves a mention

Should we support TFVars? Feels like yes - it’s kind of a “terraform-specific secret manager”

We will likely need to figure out a way to store them in the same state bucket

- see [Bucket Only - No Database](https://www.notion.so/OpenTaco-memo-2468cc53bb5a803dbb4fcf3397c0a2fc?pvs=21)

Case against: if we go “CI as compute” route then users will have secret management there

Case for: if we go with our own compute, even as option, env var management needs to be thought of. Unless we rely on K8S secrets - but then it’d be K8S only - TBD.

It feels to me though that existence of Terrakube / OTF kinda pulls us in the direction of having some solution for TFVars, even if it won’t be used by everyone. It’d just feel incomplete given that every other TACO including the OG TFC has it as a baseline.

---

# Design principles

Some already covered in [OpenTaco memo](https://www.notion.so/OpenTaco-memo-2468cc53bb5a803dbb4fcf3397c0a2fc?pvs=21) (initial product / technical hunches).

TODO merge one way or another.

### clean separation between Terraform CLI and Taco CLI

- terraform CLI is concerned with one state file, plan-apply etc
- taco CLI is for mgmt of state files + cross-state workflows (status etc)
- taco CLI might provide wrapper commands atop terraform CLI purely for convenience
eg `taco unit plan <myapp-dev>`