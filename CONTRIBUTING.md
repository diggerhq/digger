# 🤝 Contributing to Digger

Welcome to the contributing guide for Digger! We appreciate your interest in contributing to our open-source project.

**FEEDBACK:** The best way to contribute to Digger today is by using it within your organization and providing feedback. If you are considering using Digger, please [drop us a line](https://join.slack.com/t/diggertalk/shared_invite/zt-1q6npg7ib-9dwRbJp8sQpSr2fvWzt9aA), and we would be happy to set you up.

## 📚 Table of Contents

- [Introduction](#introduction)
- [How to Contribute](#how-to-contribute)
- [Coding Conventions](#coding-conventions)
- [Folder Structure](#folder-structure)
- [Submitting a Pull Request](#submitting-a-pull-request)
- [Release Process](#release-process)
- [Code of Conduct](#code-of-conduct)
- [License](#license)

## 🚀 Introduction

Digger is an open-source Terraform cloud alternative. We believe that open source software is important, and we welcome contributions from anyone who is interested in making our tool better. This document is intended to be a guide for people who want to contribute to Digger. We appreciate all contributions, no matter how big or small.

## 💡 How to Contribute

**If you are considering using Digger within your organization, please [reach out to us](https://join.slack.com/t/diggertalk/shared_invite/zt-1q6npg7ib-9dwRbJp8sQpSr2fvWzt9aA); we would be happy to help onboard you to use it.**

There are many ways to contribute to Digger, including:

- 📝 Providing feedback
- 🐛 Reporting bugs and issues
- 📚 Improving documentation
- ✨ Adding new features
- 🔧 Fixing bugs
- 🧪 Writing tests
- 💡 And more!

Before you start contributing, please read our [Code of Conduct](#code-of-conduct) to understand what is expected of contributors.

## 🎨 Coding Conventions

We strive to maintain a consistent coding style throughout the project. Please follow our [coding conventions](/coding-conventions.md) when making changes to the codebase.

## 📂 Folder Structure

```
libs/ # contains libraries that are common between digger cli and digger cloud backend (should NOT import anything from cli/pkg/ which is cli specific)
cli/cmd/ # contains the main cli files
cli/pkg/ # contains packages that are used by the cli code, can import from libs/
backend/ # contains the backend code, can import from libs/
docs/ # contains documentation pages
```


## 🛠️ Submitting a Pull Request

When you have made changes to the codebase that you would like to contribute back, please follow these steps:

1. 🍴 Fork the repository and create a new branch from `develop`.
2. ✏️ Make your changes and ensure that the code passes all tests.
3. 🧪 Write tests for your changes, if applicable.
4. 🔍 Test your changes in a demo GitHub repository using GitHub Actions.

   - You can test out the changes from your fork by referencing the Action within a GitHub workflow file: `uses: <github-username>/digger@your-branch`.
   - Fork [this demo repository](https://github.com/diggerhq/digger_demo_multienv) to set up and assert your tests.
   - If you're adding new app-level inputs that imply new environment variables, make sure to reference them **both within the `build digger` and `run digger` steps** in [`action.yml`](./action.yml).

5. 📜 Update the documentation to reflect your changes, if applicable.
6. 🔄 Submit a pull request to the `develop` branch.  
   We will review your pull request as soon as possible. Please be patient and open to feedback. We appreciate your contributions!

## 📦 Release Process

**NOTE:** The default branch `@develop` is not guaranteed to be stable, and you should always use published release versions in your testing and production environments.

- All pull requests are merged to the default `develop` branch after initial unit tests and integration tests are passing and required code review requirements are met.
- We check out a pre-release branch to prepare for an upcoming release with the pattern `prerelease-0.1.xx`.
- We perform additional manual and automated tests in this branch to ensure there are no regressions.
- Once we are ready, we tag the head of our release branch and perform a release on it.
- Tagged releases are published as actions and are the most suitable for use in production.

## 📜 Code of Conduct

We expect all contributors to follow our [Code of Conduct](https://www.contributor-covenant.org/version/2/1/code_of_conduct/) when participating in our community. Please read it carefully before contributing.

## 📝 License

Digger is released under the [Apache License](LICENSE). By contributing to this project, you agree to license your contributions under the same license.

## ⭐ Star History

[![Star History Chart](https://api.star-history.com/svg?repos=diggerhq/digger&type=Timeline)](https://star-history.com/#diggerhq/digger&Timeline)

---

<p align="center">If you find Digger helpful, please give us a ⭐️!</p>
