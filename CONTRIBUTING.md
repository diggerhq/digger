# Contributing to Digger
Welcome to the contributing guide for Digger! We appreciate your interest in contributing to our open source project.

**FEEDBACK:** The best way to contribute to Digger today is by using
it within your organisation and providing feedback. If you are considering
using Digger please [drop us a line](https://join.slack.com/t/diggertalk/shared_invite/zt-1q6npg7ib-9dwRbJp8sQpSr2fvWzt9aA), 
and we would be happy to set you up.

## Table of Contents
- [Introduction](#introduction)
- [How to contribute](#how-to-contribute)
- [Coding conventions](#coding-conventions)
- [Submitting a pull request](#submitting-a-pull-request)
- [Release Process](#release-process)
- [Code of Conduct](#code-of-conduct)
- [License](#license)

## Introduction
Digger is an open source terraform cloud alternative. We believe that open source software is important, and we welcome contributions from anyone who is interested in making our tool better.
This document is intended to be a guide for people who want to contribute to Digger. We appreciate all contributions, no matter how big or small.

## How to contribute
**If you are considering using digger within your organisation 
please [reach out to us](https://join.slack.com/t/diggertalk/shared_invite/zt-1q6npg7ib-9dwRbJp8sQpSr2fvWzt9aA) 
we would be happy to help onboard you to use it**. 
There are many ways to contribute to Digger, including:
- Providing feedback
- Reporting bugs and issues
- Improving documentation
- Adding new features
- Fixing bugs
- Writing tests

- And more!
  Before you start contributing, please read our [Code of Conduct](#code-of-conduct) to understand what is expected of contributors.

## Coding conventions
We strive to maintain a consistent coding style throughout the project. Please follow our [coding conventions](/coding-conventions.md) when making changes to the codebase.

## Submitting a pull request
When you have made changes to the codebase that you would like to contribute back, please follow these steps:
1. Fork the repository and create a new branch from `develop`.
2. Make your changes and ensure that the code passes all tests.
3. Write tests for your changes, if applicable.
4. Update the documentation to reflect your changes, if applicable.
5. Submit a pull request to the `develop` branch.
   We will review your pull request as soon as possible. Please be patient and open to feedback. We appreciate your contributions!

## Release Process
**NOTE: The default branch `@develop` is not guaranteed to be stable and you should always use published release versions in your testing and production.**

- All pull requests are merged to the default develop branch after initial unit tests and integration tests are passing and required code review requirements are met.
- We checkout a pre-release branch to prepare for an upcoming release with the pattern `prerelease-0.1.xx`.
- We perform additional manual and automated tests in this branch to make sure there are no regressions.
- Once we are ready we tag the head of our release branch and perform a release on it.
- Tagged releases are published as actions and they are the most suitable to be used in production.


## Code of Conduct
We expect all contributors to follow our [Code of Conduct](https://www.contributor-covenant.org/version/2/1/code_of_conduct/) when participating in our community. Please read it carefully before contributing.

## License
Digger is released under the [Apache License](LICENSE). By contributing to this project, you agree to license your contributions under the same license.
