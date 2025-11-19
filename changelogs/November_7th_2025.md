# 11/07

We’ve rebranded the Digger project to OpenTaco! It’s taken a LOT of work to get here, and a lot more to be worked on but onwards we move! This will gradually be incorporated in all of the Digger project’s existing marketing material and repositories, to reflect the same.

The decision to rebrand wasn’t taken lightly. Over time, we’ve heard the same question again and again: is there an open-source, fully self-hostable TACOS tool built for the Terraform and OpenTofu ecosystem? OpenTaco is our answer to that.

But this isn’t a rebrand announcement, it is our changelog for the week, so here’s the stuff we shipped this week:

# Changes to Speed up the UI #2385

@breardon2011’s PR Significantly improved UI and API performance by parallelizing expensive calls, adding composite indexes across units/roles/permissions/tags, introducing optimistic resource creation, and reducing repeated org lookups through enriched organization context. 

The PR also Added connection-pool tuning and prepared-statement support for MySQL/Postgres/SQLite, plus gzip/caching improvements and loading skeletons for slow paths. 

Unit creation now initializes S3 blobs asynchronously for faster responses, and logging was cleaned up across middleware and handlers. 

# Onboarding Flow #2380

This PR by @motatoes improved the unit onboarding flow with clearer first-time guidance, proper handling of the “0 units” case, added last-updated metadata, introduced a copy-to-clipboard button for the digger.yml snippet, and applied general UI polish including updated spacing and styling across the create-unit modal and onboarding screens. We’re super excited to reduce as much friction as possible from sign in to first plan.

# Make create modal prettier #2377

More UI polish! This PR refined the “Create Unit” modal UI with updated spacing, cleaner styling, and improved alignment for a sleeker onboarding experience across the unit creation flow. If you’re reading this and end up going through the unit creation flow, please feel free to share feedback on the community slack channel!

Aside from the above, a bunch of docs improvements were done to make the onboarding experience smoother. A lot of the docs and marketing material are still a WIP, but check out the launch video below for an update of what we’ve been working on in a nutshell and PLEASE give us feedback: https://www.linkedin.com/feed/update/urn:li:activity:7392614043494232064/
