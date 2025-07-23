# Mintlify documentation

## Working relationship

- You can push back on ideas-this can lead to better documentation. Cite sources and explain your reasoning when you do so
- ALWAYS ask for clarification rather than making assumptions
- NEVER lie, guess, or make up information

## Project context

- Format: MDX files with YAML frontmatter
- Config: docs.json for navigation, theme, settings
- Components: Mintlify components

## Content strategy

- Document just enough for user success - not too much, not too little
- Prioritize accuracy and usability of information
- Make content evergreen when possible
- Search for existing information before adding new content. Avoid duplication unless it is done for a strategic reason
- Check existing patterns for consistency
- Start by making the smallest reasonable changes

## Frontmatter requirements for pages

- title: Clear, descriptive page title
- description: Concise summary for SEO/navigation

## Writing standards

- Second-person voice ("you")
- Prerequisites at start of procedural content
- Test all code examples before publishing
- Match style and formatting of existing pages
- Include both basic and advanced use cases
- Language tags on all code blocks
- Alt text on all images
- Relative paths for internal links

## Git workflow

When user says "let's push to git" or similar:
1. Check git status first
2. If on develop or main, create a new branch using convention:
   - `docs/` for documentation changes
   - `feat/` for new features
   - `fix/` for bug fixes
3. Stage and commit changes with one-line message
4. Push with `-u` flag to set upstream: `git push -u origin branch-name`
5. Default branch is `develop` (not main)

Standard workflow:
- NEVER use --no-verify when committing
- Ask how to handle uncommitted changes before starting
- Create a new branch when no clear branch exists for changes
- Commit frequently throughout development
- NEVER skip or disable pre-commit hooks
- Keep commit messages to one line unless user requests otherwise

## Do not

- Skip frontmatter on any MDX file
- Use absolute URLs for internal links
- Include untested code examples
- Make assumptions - always ask for clarification
