# The case for OpenTaco

OpenTaco is an open-source alternative to Terraform Cloud & Terraform Enterprise (known as [TACOs](https://itnext.io/spice-up-your-infrastructure-as-code-with-tacos-1a9c179e0783). These products solve [the pains in terraform collaboration](https://itnext.io/pains-in-terraform-collaboration-249a56b4534e). Other TACOs exist (eg Spacelift, Env0, Scalr, Terramate, Harness) but none of them are fully open source. A number of active open source projects address some of the pains (Atlantis, Digger, Terrateam) but they are not fully-featured TACOs - they only solve PR automation.

## Terraform and OpenTofu

Terraform is an infrastructure-as-code CLI tool that allows to provision and configure cloud infrastructure - such as AWS. It was created by Hashicorp, initially under MPL 2.0 license. In 2023 Hashicorp changed the licens and OpenTofu ws launched as an MPL fork of Terraform. It is governed by Linux Foundation and sponsored by TACOs vendors.

- OpenTofu is an MPL-licensed alternative to Terraform the CLI after Hashicorp changed its license to BSL.
- OpenTaco is an open-source alternative to the commercial servers (TACOs) that are used to run Terraform / OpenTofu CLIs. Those were never open source.

## Key aspects of a TACO

- **Management of state files**. Both Terraform and OpenTofu rely on state files which, if not stored securely, can be easily lost or corrupted.
- **Centralised access controls**. You don't want all developer machines to have unrestricted access to cloud accounts.
- **Remote execution**. Developer machines don't have access to cloud accounts (see above) but terraform / opentofu CLI still needs to run somewhere. TACOs run terraform / opentofu as isolated server-side jobs.
- **CI/CD and gitOps**. Changes to terraform code in the source control system (e.g. GitHub) need to be automatically applied.
- **Drift detection**. Direct changes to infrastructure that bypass IaC are detected, reported and remediated
- **Policy as code**. Changes to infrastructure code are checked against policies (e.g. OPA), which themselves are stored as code

## PR automation tools vs TACOs

Back in 2017 Luke Kysow [built](https://medium.com/runatlantis/introducing-atlantis-6570d6de7281) Atlantis - the first tool that allowed to run Terraform via comments in a pull request (PR automation). In 2018 Luke joined Hashicorp, and in 2019 Hashicorp launched Terraform Cloud - the first TACO by Hashicorp. In 2020 Spacelift launches the first alternative TACO. Later, other PR automation tools were created that improved on security and performance of Atlantis - namely Terrateam and Digger.

PR automation tools are primarily concerned with the CI/CD aspect. They introduce a workflow unique to terraform: user comments "atlantis apply" or "digger apply" in an open pull request, and an apply job starts - _before_ the PR is merged. This approach allows for faster iteration, because apply runs often fail; with PR automation tools you don't need to create a new pull request for every change. But this is also against the conventional wisdom of CI/CD best practices (changes should only be deployed after the code is merged).

TACOs such as Terraform Cloud or Spacelift take the conventional approach to CI/CD: changes are only applied after the PR is merged. TACOs also address other aspects that enterprises care about: state management, remote execution, drift detection, policy-as-code.

## What if there was an open-source TACO?

So today we have:
- open-source-ish IaC CLIs (terraform / opentofu)
- commercial SaaS TACOS to run these CLIs, the most popular being TFC by Hashicorp and Spacelift
- open-source PR automation tools (Atlantis, Digger, Terrateam) 

Now also consider this:
- Terraform is used by millioins of developers, it's the de-facto industry standard for IaC.
- HCL is the fastes growing language on GitHub for 3 years in a row ([2022](https://octoverse.github.com/2022/) [2023](https://github.blog/news-insights/research/the-state-of-open-source-and-ai/) [2024](https://github.blog/news-insights/octoverse/octoverse-2024/))

What would the world look like if Hashicorp made the rest of the Terraform stack open-source, including the TACO backend that's needed to run it in a team setting? Would anyone have any reason to use anything else? But Hashicorp of course wouldn't do it. They even switched the license of Terraform itself, and now we have 2 competing IaC CLIs (BSL terraform and MPL opentofu).

But what if someone else does it? Several things will happen:
- Every new user of Terraform / OpenTofu would have zero reason to use anything but OpenTaco
- Existing paying customers of TFC will have a strong reason to migrate (in addition to the license change Hashicorp also switched pricing to resources-under-management which was not well received; people are already looking for alternatives
- Alternative commerical TACOs like Spacelift instantly become irrelevant 

Curiously, the right home for something like this seems to be the OpenTofu project - but it will never happen there, because it is backed 100% by the commercial TACO vendors! So the opportunity window is wide open right now for someone else to do it.

# How will this thing make money?

[Something like that](https://diggerdev.notion.site/How-will-this-thing-make-money-v2-0-24c8cc53bb5a8038b348d7c914aceaf1?source=copy_link)

# How will it work?

[Something like this](https://diggerdev.notion.site/OpenTaco-memo-2468cc53bb5a803dbb4fcf3397c0a2fc?source=copy_link)


