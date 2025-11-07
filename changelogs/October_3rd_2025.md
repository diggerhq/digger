# Week ending 10/3

Welcome to the weekly Digger changelog, where we share product updates, tool tips, and more. This week we're building on top of the v0.0 state manager piece for OpenTaco, improving the “getting started experience” and the laying the groundwork for v0.1, all this while shipping improvements/enhancements to the core product.

Let’s dive right in

Latest version: **v0.6.127**

1. We’ve had a whole host of new contributors. 

    - [**@Utwo**](https://github.com/Utwo) made their first contribution in [#2280](https://github.com/diggerhq/digger/pull/2280)
    - [**@Juneezee**](https://github.com/Juneezee) made their first contribution in [#2281](https://github.com/diggerhq/digger/pull/2281)
    - [**@sidpalas**](https://github.com/sidpalas) made their first contribution in [#2278](https://github.com/diggerhq/digger/pull/2278)
2. We added an interactive setup wizard for the OpenTaco CLI so users don’t need env vars to point at the server anymore. This was a pain we observed while on a user call and prioritised fixing it on priority:

      Net effect
      
      - First-run onboarding: users get a guided prompt instead of having to set env vars.
      - Persisted config: fewer flags needed each run; you can re-run `taco setup` anytime to change it.
      
      More in [#2268](https://github.com/diggerhq/digger/pull/2268)
3. Fixing behaviour when Draft PR is open: Notes from the author, @motatoes
    
      We hadn't been correctly behaving when a draft pull request is opened. This PR fixes the behaviour to avoid unnecessary locking on newly opened draft PRs. For older draft PRs that would be having dangling locks we explicitly react to "digger unlock" comment to clear all locks . Screenshot compares behaviour with current develop branch (digger pro) and the local changes (digger local)
      
      This is a PR that was opened as a normal PR following a previous draft PR opened which touches the same project:
      
      Expected behaviour is that the pr should plan normally no issues. Current behaviour (digger pro) is that it fails to plan because locked by another pr
      
      This behaviour is for existing PRs that would be opened as drafts and have a dnaling lock present. In current develop branch the "digger unlock" comment on the other PR would be ignored so it would not unlock and the other PR would be stuck and not able to aquire the lock. In this PR we now force unlock even when it is a draft PR configured to be ignored so the digger unlock will allow the user to plan on the other PR that was blocked by the draft PR
      

4. Smoother `taco login` with Microsoft/Entra, and copy-pasteable paths to run OpenTaco Statesman on AWS with HTTPS out of the box: More here: [#2284](https://github.com/diggerhq/digger/pull/2284)
5. GCP (Cloud Run) quickstart guide for deploying Statesman with Artifact Registry + Cloud Run: [#2275](https://github.com/diggerhq/digger/pull/2275)

      Includes:
          - Example shell script for pushing the Docker image and configuring Cloud Run.
          - Auth0 setup instructions with callback URL and screenshots.
    
      What Changed:    
      - Docs updated to reflect new CLI flow (`taco setup` / `taco login`) instead of manual env var export.
      - Navigation updated (`mint.json`) to include the new GCP quickstart page.
