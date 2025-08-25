# OpenTaco memo

[Moonshots (out of OpenTaco scope)](https://www.notion.so/Moonshots-out-of-OpenTaco-scope-2498cc53bb5a80db9522c54e2b094746?pvs=21)

[ICP of atlantis vs TFC](https://www.notion.so/ICP-of-atlantis-vs-TFC-24d8cc53bb5a800397c0dcaeafbeb2ce?pvs=21)

# North stars

- Mitchell-worthy (this doc to be shared with Mitchell)
- Nothing else needed to work with Terraform / Tofu (c) Utpal
- Config management - cloud providers are not the only targets (Datadog, Github, …)
- We’re good enough for Atlantis but not nearly ready to become replacement of TFC (c) Mo
- End goal (not done until it’s achieved): get ppl move from TFC to OpenTaco (c) Mo

---

below this raw ideas in no particular order

---

[TACOS space](https://www.notion.so/TACOS-space-24c8cc53bb5a805986c3d305533c35c3?pvs=21)

# What other tools exist (both commercial and oss)?

We start with the space of TACOs. We can bucket them under multiple dimensions: OSS vs not, before-merge focused vs after-merge focused

# Get users early in their terraform journey

First attempt: [hierarchy of TACO needs](https://docs.google.com/presentation/d/1l5XaXx2NACUNxY7zAaV8xW3OBk6i2Izd7gXgHNF5BYU/edit?usp=sharing)

Ideally, people should be starting using Digger *at the same time* as they first touch Terraform or OpenTofu. What would be the hook / trigger? TBD, to be explored separately. For now I just want to put into words WHY is it critical to be early.

Let’s consider the following well-known entry points:

- First start with terraform (might be too early - Mo)
- Need remote state (← we enter here)
- Moving away from a laptops to centralise access control
- Need CI/CD (this is where Digger currently is)
- Move from TFC (Mo)

The later on this scale the user is, the harder it is to convince them to use a third party, and the more it is likely that they already have some sort of a solution in place (DIY / another TACO).

Process (internal): if we fix ICP on late entry point, then product and eng work for the earlier stages is the strict subset of the work required for the later stages, so it makes sense to start at the left.

# Don’t compete with TACOs head-on.
Become an indispensible toolkit for TF / Tofu.

TACO (automation / collaboration) is much further downstream, months or years after users are already using Digger with Terraform or OpenTofu. When users hit one of the problems solved by TACOs they discover that they don’t need anything else, they already have Digger.

# State *control* (not managed state)

The subtle difference between “managed state” and “state control” is key to understand.

State control means ability to allow or deny each operation to state. But that doesn’t mean that the state is stored behind a black-box managed API. It is likely that people actually prefer to keep a bucket or several buckets - it’s not hard and gives additional degrees of control / security / debuggability (automated backups, versioning, manual editing of every file, etc).

# Git inspired state management

Every terraform root module is viewed as a system that receives inputs and produces outputs. The outputs are stored in the statefile. If we build launch a simple state proxy which also captured diffs as commits everytime the state changes  we are able to identify every change to outputs. But outputs in many cases don’t change. Hence the git inspired, whenever any of the outputs changes we had a new “commit”. This state management piece needs to be able to answer this question: have these set of outputs changed between commit A and commit B? The reason this is important is the next section

# Provider to read versioned inputs

With outputs properly versioned, we can now have a root module which reads the outputs of another root module — but versioned. S-o every read of the outputs needs to be tracked on the consumer side, the last time it was read for an apply. Given the version of each output used and the read date for every consumer of the output we can combine these two questions to answer whether a module needs to be applied or no *

*That is assuming that no other resources have changed, which is the idea of propagating dependencies

# Drop-in add-on on top of an existing state bucket

Respecting existing conventions / bucket layouts (eg terragrunt) and finding a way to only add optional files would make adoption much easier.

# State of states

First attempt to verbalise this idea: [State of States / Outputs propagation](https://docs.google.com/presentation/d/1iQQd8fzW47qVl0Qv6pRQtm4MaZqtqPPr91pepAkixQI/edit?usp=sharing)

Second attempt: [the case for standalone state backend manager](https://www.reddit.com/r/Terraform/comments/1l48iyf/the_case_for_a_standalone_state_backend_manager/)

The point is that the collection of individual projects / statefiles is itself a stateful system. For example, when apply on one of the states results in new outputs that are used in other states, this information is completely lost, the only way to know it is to remember what you did and intended to do. 

These stateful bits (past outputs or full history) need to be stored (most likely in the bucket) and be easily queryable (by both UI and CLI → meaning it’d need a clean API). And we’d need some sort of a “workflow” on top of single apply operations - smth like “batch” or “futures chain” that’d capture the *intended graph walk* and progress along the way. So that user could see where they are at (see node coloring in the deck) and then also cancel if it’s no longer relevant. First and foremost we’d need to provide visibility

# Bucket only - no database

This is counterintuitive, but I think we’d win A LOT by finding a way to contain all stateful parts of the system in the bucket. It’d make adoption much easier (not much to deploy - and we faced some pushback already on setting up dedicated postgres); makes the deployment effectively free (dbs are expensive, whereas a lightweight stateless service in K8S is nearly free).

We can have some resemblance of a “db file” (similar to sqlite) in the bucket.

But no in-memory DB god forbid! The service should be assumed to be ephemeral, multiple instances could always be running, and it could die anytime. In-memory state of any kind is a big no-no. The only stateful piece in this system is the bucket (S3 or GCS or otherwise).

It doesn’t need to be fast!!! TACOs are relatively low-throughput pieces of software; and the slowest part is terraform itself (and it’s not even using CPU - most of the time it just waits for requests by providers).

# CLI first, UI later

UI feels like a MUST for OpenTaco. Without UI it’s not a thing. But I think it’s wise to start with a clean API + CLI combination - mainly to figure out *what exactly it should do,* and get the API design right. This’ll force us to come up with a clean, well-tested API - and then UI will be also much easier to design on top because the semantics is settled. The API-first approach was new and cool back in 2010s but today it’s the baseline, especially for deeply technical infra pieces.

# Cleanly abstracted compute

“multi-ci” and “multi-VCS” are very different semantically. I think we also got the terminology slightly off and that led to some skewed thinking and bad design as a consequence.

Multi-CI is about supporting multiple different CI platforms for executing jobs. There’s no variation whatsoever in how these work - just start the job; the only difference is APIs of each CI provider. We even report back in a unified way from the CLI, there’s no Github-specific reporting! So the way it should’ve been done is the orchestrator *unaware* of which CI actually runs the job. Webhooks or perhaps basic adapters cleanly separating orchestrator from the executor. If it’s done this way we can make one adapter (github) and let the community build other adapters if they like.

When we say “VCS” we imply many different things actually:

- comment-ops UX
    - user-to-digger commands via comments
    - digger-to-user responses via comments and reactions
- VCS-specific APIs
    - status checks
    - merge queue

Comment-ops, just like “multi-ci”, should be abstracted into neat adapters; we build one (only for github) and let the community build the rest. For VCS-specific APIs though it may not be as straightforward - we’d need to introduce a layer of digger-specific events / subscriptions for the VCS-specific adapters to consume without polluting Digger domain with plugin-specific terminology. This is much harder than “multi-ci” because there’s a lot of nuance. So it’s very reasonable to only think in terms of Github for the foreseeable future; but we MUST get the baseline abstractions right so that later on community can build a gitlab-specific adapter and *not a single line of code* in “digger core” would need to change. It’s just basic sanity of system design, we need to start doing it.

Local execution (of terraform) is no different in principle from remote execution; the only difference is access / security. In some sense there’s a world where user chooses to run terraform locally but needs the orchestration / ordering part from Digger. A bit like Terragrunt. In that case they’d just use smth like “local runners”; We need to think of the right abstraction to make it possible (but not necessarily build it right away - remote-only is a reasonable starting point).

# Local execution = easier adoption

See the point about abstracting compute. we need to think of a clean compute abstraction to allow both remote and local execution of terraform / tofu, but it doesn’t look like we have to build local right away.

However, making it possible to run everything locally, without a server or anything, makes it even easier to adopt Digger. Imagine if we manage to defer all the complexity for later, and the most basic scenario is just a CLI that tracks plan / apply order and dependencies, like Terragrunt. But it has easy-to-follow paths to scaling up. Need state managed? Just run “digger login” and “digger import” (it’d move the state file to digger cloud in the background). Need to restrict local execution so that all jobs run remotely by default? Run some other config / permissions command.

# Replacing Terragrunt: “multiplayer terragrunt”

The long-term goal should be that people don’t see use in smth like Terragrunt. Realistically there’s only one spot in the “terraform companion” land, and it’s currently occupied by Gruntwork. Expecting people to choose three tools - terraform + terragrunt + digger - is just naive. Some will ofc, but simply because it’s 3 separate choices the probabilities are multiplied and we get a much smaller market.

The proposition for this replacement is very clear: Digger improves on Terragrunt by introducing a stateful dependency graph that relies on the same bucket that’s already used for state. So you can pick up from where you stopped; and any of the teammates can (nothing is lost, all state is stored in the same bucket. see above re bucket / state of states).

We’ll likely need to make sure that a pure “local CLI + bucket” combination is fully operational and creates enough value for people to ditch terragrunt (see local execution). No backend!

From the purely local mode the user needs to be able to smoothly upgrade to “guarded mode” where the user cannot run any commands against the state locally, remote is the only way. For example, if user logs in, or self-hosts the backend with some form of compute attached (ci or K8S shouldn’t matter, see “cleanly abstracted compute”), then they get upgraded to the guarded mode and local execution is no longer possible.

# Faster Terragrunt

Terragrunt has done amazingly at allowing modules to be connected together and map inputs to outputs. The biggest issue with terragrunt is how slow it becomes with large amount of modules. One reason for its slowness is that there is no easy way to propagate the graph. We have run-all. And even with things like terragrunt-atlantis-config what we get in the end is blind propagation of the graph with unnecessary plans for so many modules that will have “no resources changed”. I believe with the above versioned outputs and versioned input consumption, we will be able to do smarter propagation hence delivering much faster change detection for any set of impacted modules.

# Backend and CLI are both “clients of state”

In some sense the backend (state manager, possibly orchestrator too) and the locally-run CLI are both consumers of the stateful component (state of states and states themselves in the bucket). If we for a second ignore the need to disallow direct access to the bucket, we can imagine a scenario where 2 engineers are trying to run something, one locally, another remotely; and the only conflict resolution mechanism is the state bucket.

This line of thinking needs to be developed further or discarded; cannot see implications yet.

![photo_2025-08-06 19.37.08.jpeg](attachment:3dde4d87-4ccc-4b7a-81ab-4f28d68bf2a9:photo_2025-08-06_19.37.08.jpeg)

# Auth is not optional

And it’s not an enterprise feature either. We need to figure a model that’d work just as well at small scale as it would at enterprise scale with dozens of organisations (github orgs), each with thousands of state files.

- Third party SSO/SAML support *without cloud tools (eg workos)*
- Our own cloud-hosted version should use the exact same code - but configured to use workos as auth provider. It’s absolutely doable.
- No auth should be supported too
- CLI auth also needs to be thought through properly

# Irreducible broadness in state mgmt

As a rule of thumb I think we should definitely focus on a *singular* adoption point / use case. But some aspects I think have this strange property that, if we only take one use case, then the optimal solution is a dead end.

State management is one of those. It makes sense to focus on only one or only the other:

- User already has state managed → ignore, don’t touch it at all
- User needs to put state somewhere → provide a cloud-managed seemless state solution

But both are wrong imo. Because each user sooner or later will go through all stages of complexity. We need to meet users at whichever stage they are, and help them go through further stages smoothly.

- Local state: allow the user to import it (same for cloud or self-hosted)
- State already in a bucket:
    - self-hosted: allow to import the bucket (becomes one bucket for all states)
        - later: support multiple state buckets
    - cloud: allow to import the state files themselves (under the hood we create a bucket per org)

# Self-hosted first

We should treat building the OSS self-hostable OpenTaco as a product discovery exercise, and optimise for making the right thing first and foremost. Making a well-designed TACO multitenant is much easeir than making a SaaS-first multitenant TACO self-hostable. Completely different choices:

- Store (see “bucket only - no Postgres”)
- Auth (workos anyone? clearly we need our own auth harness and treat workos as a plugin)
- Compute (see “cleanly abstract compute”)

We must not get locked into any cloud only solution (eg Cloudflare). K8S-native is the way these days; could also be good old VM image for legacy deployments, but definitely not no-standard deps

Also see [Bucket only - no database](https://www.notion.so/Bucket-only-no-database-2478cc53bb5a80f7adb0ed4b27425588?pvs=21) - we’d benefit from as little moving parts as possible and if we can ship 1 ephemeral container for API + 1 for UI + user-provisioned bucket that’d be AWESOME

# User can run the server locally too!

Elegant solution for the need to sometimes run things laptop-only without having to manage a server anywhere: run both CLI and API locally!

This way we can keep it clean: CLI only talks to the API (never direct bucket access); only the API can have direct access to S3 (through env vars). And it’s the exact same API that the user hosts in their K8S cluster or hosted in digger cloud version! (state manager / orchestrator). Only the API server can modify state; CLI can either call the API server, or “accept a job” from the API server (to run terraform / tofu command which in turn will use the API server as state proxy). So the CLI *never runs terraform directly*; only when the API server tells it to do so (dispatches a job).

Running both CLI and API server locally would be the only way 

We should ship the API with CLI somehow, or maybe make it as easy as “npx digger-server”. It shouldn’t be weird installation / configuration to get it running; it should feel feather-light (it actually is! it’s just a job dispatcher / state proxy).

# Don’t build PR automation yet

We should resist the temptation to quickly get to parity with current Digger.

This is a known territory and easiest to build. But I worry that it’d lead us astray.

All there is to PR automation is a special quirky kind of a CLI-like UI but hosted by Github.

We should initially focus on making a clean CLI + API that work in baseline scenarios (plan / apply locally; remote execution; dependencies;  RBAC - order TBD). Only after this “core” works, we can slap VCS-specific integrations onto the API - effectively extending a subset of CLI UX to the PR comment thread.

PR automation and Dashboard and CLI are 3 flavours of the UI. We’ll get to it; the most important thing is the core, how it works under the hood. One CLI is sufficient to get it working and is quickest to iterate on. After the CLI flow is settled, we expand UI surface to PRs and Dashboard.

# Separate service(s) for repo ops

We should not attempt to mix the unmixable - service boundary best practices exist for a reason

A service that responds to requests from CLI / UI / github has a completely different compute, memory and FS profile compared to smth that checks out the repo and does some analysis. Mixing them in one application is the same as telling people that we are clueless.

Repo checkout is more of a pipeline usecase; we might end up having more than one such “worker” service. It is not latency-sensitive so the right compute for deploying it would be something serverless, and cold starts are OK. Could be cloudflare containers, google cloud run, this sort of stuff. Could be edge as well, it does not matter how close it is to DB geographically because git is the bottleneck for this. It needs either fast FS or lots of memory for in-memory FS; and it needs to be ephemeral (each invocation has pristine file system so that we don’t have to cleanup tmp files by hand or make guid names like it’s 2005).

API on the other hand is highly sensitive to latency; and it is not ideal to deploy it on edge runtimes because it needs to be close to DB (in our case S3 bucket but still). API is not doing much CPU- or FS- or memory-intensive stuff, it just handles requests and triggers webhooks. It should not directly connect or be aware of other services (not even gh) but instead trigger events in the queue (which in our case would also be S3 bucket as default impl; we probably should make adapters for postgres / redis / SQS / whatever)

# Github compute by default (no built-in runners)

We should start with what we know because it’s not consequential, especailly in OSS self-hosted.

And there are major advantages in security / scalability that ppl like us for. “reuse your CI” is kinda sensible.

The right TACO would still use existing CI for compute - and that is separate from AAM / ABM.

But we must think of compute as a separate and cleanly abstracted concern

See [Cleanly abstracted compute](https://www.notion.so/Cleanly-abstracted-compute-2478cc53bb5a801784b4e284f59f9ff0?pvs=21) 

# Plan Preview is not the same as PR Automation

Apply in PR is somewhat controversial (ABM vs AAM); but showing the plan is the basics (and all AAM tacos have it too).

So OpenTaco MUST have a PR comment functionality

Whether or not to allow to apply before merge, that’s a separate question

# ABM as optional feature that can be enabled

It makes sense to follow TFC and Spacelift practice here

By default it’s AAM - merging means it’s correct and apply jobs are kicked off. This way merge queue also does its job as intended.

But if the user wants ABM then they can enable it for 

# Provider to control Opentaco itself

from Jumpcloud list but also kinda obviously needed

# TFVARs in UI

from Jumpcloud list and previous attempts: [Handling of TFVars](https://www.notion.so/Handling-of-TFVars-0df739e702a64ef6a52e4d4ecb9dda54?pvs=21)