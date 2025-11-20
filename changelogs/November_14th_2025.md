# 11/14

We’ve been having a ton of sign ups on [otaco.app](http://otaco.app) after the launch, and also went live at our first conference earlier this week, we were diamond sponsors at OpenTofu day at Kubecon in Atlanta. It was quite an experience speaking to folks who had used OpenTaco already and was also interesting to learn about painpoints folks see in existing tooling. Below is an image of the booth!

<img width="1600" height="1200" alt="Image" src="https://github.com/user-attachments/assets/fa07118b-ae9c-48f4-bd41-e599cf9277b8" />

Now to move onto what we shipped this week:

# **Remote runs POC (backend)  #2451**

This PR implements the backend foundation for "Remote Runs," allowing OpenTaco to manage Terraform plan and apply executions directly. It introduces new database structures and TFE-compatible API endpoints to track runs, plans, and configuration versions, alongside dedicated executors that handle Terraform operations with automatic state locking. The update also adds secure log streaming, extends unit configuration with TFE-specific settings like execution mode and auto-apply, and includes the necessary database migrations and feature flags to support the rollout of managed remote execution.

# Support caching of impacted projects #2452

#2452 introduces a database-backed caching system to track "impacted projects" for each commit, improving the reliability of CI/CD workflows. It adds a new ImpactedProject model and corresponding database tables to persist which projects are affected and their execution status (planned vs. applied), replacing ephemeral in-memory checks. The update modifies PR and comment event handlers to populate this cache automatically and updates the auto-merge logic to verify project status directly from the database, ensuring more accurate validation before merging.

We also had a whole host of fixes, something we’d expected after a blockbuster launch week!
Full commit history is [here](https://github.com/diggerhq/digger/commits/develop/). If you do end up trying out OpenTaco v0.2, please do share your thoughts, we’re very hungry for feedback!
