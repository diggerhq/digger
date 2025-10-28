## Week ending 10/10

Welcome to the weekly Digger changelog, where we share product updates, tool tips, and more. This week we're continuing our efforts towards the v0.1 launch of OpenTaco, below is a summary of what we worked on this week!

Latest version: **v0.6.128**

- We had one new contributor this week!
    - [**@golemiso**](https://github.com/golemiso) made their first contribution in [#2286](https://github.com/diggerhq/digger/pull/2286). Previously, include_patterns and exclude_patterns only worked with absolute paths, which made configurations less flexible. Supporting relative paths improves usability and aligns with common developer expectations. Thank you for your PR @golemiso!

- @sidpalas in #2283 added support for an optional digger-version input which enables pinning usage of the digger action to the commit hash without incurring the performance penalty of needing to rebuild from source.
- We’re super keen on reducing the “time to first plan” with Digger, and some docs changes were made to reflect this: checkout “streamline getting started guide” by [**@s1ntaxe770r**](https://github.com/s1ntaxe770r) in [#2302](https://github.com/diggerhq/digger/pull/2302)
- Enable configurables for the cli os/arch by [**@breardon2011**](https://github.com/breardon2011) in [#2311](https://github.com/diggerhq/digger/pull/2311)
    - Adds two new optional inputs to the GitHub Action—`digger-os` and `digger-arch`—and wires them into the CLI download logic. Previously the action always used `runner.os` / `runner.arch`; now you can override them. [GitHub](https://github.com/diggerhq/digger/pull/2311/files)
    - New inputs **in `action.yml`:**
        - `digger-os` (valid: `windows`, `linux`, `darwin`, `freebsd`; default `"Linux"`)
        - `digger-arch` (valid: `amd64`, `arm64`, `386`; default `"X64"`) [GitHub](https://github.com/diggerhq/digger/pull/2311/files)
    - How it’s used**:** The PR sets `DIGGER_OS` / `DIGGER_ARCH` env vars and updates the download URL construction to use these inputs instead of the runner’s values.
    - Why**:** Fixes a release naming mismatch and lets you fetch a prebuilt CLI for a specific OS/arch (useful for cross-fetching artifacts).
- Change "fallback" to skip under fallback condition by [**@breardon2011**](https://github.com/breardon2011) in [#2312](https://github.com/diggerhq/digger/pull/2312)
    - Replaced fallback logic for missing branches with graceful handling that safely skips runs when a branch is deleted post-merge.
    - Introduced ErrBranchNotFoundPostMerge to clearly identify and handle this condition without triggering errors or unnecessary fallbacks.
    - Removed the legacy “fallback to target branch” logic, simplifying post-merge workflows and reducing redundant repository clones.
