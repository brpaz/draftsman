
# Contributing Guidelines

Thank you for your interest in contributing to the project!
This document outlines the guidelines and best practices for contributing to the project. Please take a moment to read through it before submitting a pull request.

## Table of Contents

- [Ways to Contribute](#ways-to-contribute)
- [Reporting Issues and Feature Requests](#reporting-issues-and-feature-requests)
  - [Before Opening an Issue](#before-opening-an-issue)
- [Contributing with Code](#contributing-with-code)
  - [Setup the Development Environment](#setup-the-development-environment)
    - [Starting Devenv Shell](#starting-devenv-shell)
    - [Running common development tasks](#running-common-development-tasks)
    - [Code quality tools](#code-quality-tools)
    - [Development Workflow](#development-workflow)
    - [Conventional Commits](#conventional-commits)
    - [Pull Request Checklist](#pull-request-checklist)
    - [Git Commit Hooks](#git-commit-hooks)
- [Review Process](#review-process)
- [Release Process](#release-process)

## Ways to Contribute

Contributions can take many forms, not just code. Here are some ways you can contribute:
- Reporting bugs or suggesting features by opening issues.
- Submitting pull requests with bug fixes or new features.
- Improving documentation.
- Reviewing and providing feedback on existing pull requests.
- Helping other users in the community.

## Reporting Issues and Feature Requests

Issues and feature requests can be reported by opening a new issue on [GitHub](/issues).

Please use the appropriate templates when opening an issue:

- **[Bug Report](https://github.com/brpaz/draftsman/issues/new?template=bug-report.yml)**: For reporting bugs or unexpected behavior
- **[Feature Request](https://github.com/brpaz/draftsman/issues/new?template=feature-request.yml)**: For suggesting new features or improvements

### Before Opening an Issue

- Search existing issues to avoid duplicates
- For security vulnerabilities, follow the [security policy](./SECURITY.md) instead

## Contributing with Code

If you are a developer and want to contribute code, please follow the guidelines below to ensure a smooth contribution process.

### Setup the Development Environment

This project uses [Devenv](https://devenv.sh/) to provide a self-contained development environment using the power of [Nix](https://nixos.org/).

It´s recommended to use Devenv to ensure a consistent development environment between different contributors. To install Devenv, follow the [Getting Started](https://devenv.sh/getting-started/) instructions at Devenv´s website.

[Direnv](https://direnv.net/) is also recommended. Direnv allows to automatically load the Devenv environment when you `cd` into the project directory, as well as simplify the management of project level environment variables. Follow the instructions at [direnv.net](https://direnv.net/docs/installation.html) to install Direnv and to integrate it with your shell.

### Starting Devenv Shell

```bash
git clone https://github.com/brpaz/draftsman.git
cd draftsman
direnv allow
```

A devenv shell should be automatically loaded, after running `direnv allow`, if you have Direnv installed and properly configured.

You can verify that you are in the Devenv shell by checking the output of  `which go`. If you see a path like `/nix/store/.../bin/go`, then you are in the Devenv shell.

If you are not using Direnv or if your shell was not automatically loaded for some reason, you can manually start the Devenv shell by running:

```bash
devenv shell
```

### Running common development tasks

The project uses [Taskfile](https://taskfile.dev/) as a task runner to simplify the execution of common tasks during development, like running linters or tests.

To run a task, use the following command:

```bash
task <task_name>
```

Some of the included tasks are:

| Task Name  | Description                            |
| ---------- | -------------------------------------- |
| build      | Build the project                      |
| test       | Run unit tests with coverage           |
| lint       | Run GolangCI-Lint                      |
| lint-fix   | Run GolangCI-Lint with auto-fix        |
| gomod      | Download Go modules and tidy           |
| gomarkdoc  | Generate documentation using gomarkdoc |
| docs-build | Build documentation site               |
| docs-serve | Serve documentation locally            |

You can list all available tasks by running:

```bash
task -l
```

### Code quality tools

This project uses the following tools to ensure code quality and consistency:

- [GolangCI-Lint](https://golangci-lint.run/) - A fast Go linters runner that runs multiple linters in parallel and provides a unified output.
- [Gotestsum](https://github.com/gotestyourself/gotestsum) - A test runner that provides a more readable output for Go tests, including test summaries and support for test sharding.
- [Commitlint](https://keisukeyamashita.github.io/commitlint-rs/) - A tool to lint commit messages according to the Conventional Commits specification.

### Development Workflow

1. Create a new branch for your feature or bug fix:
   ```bash
   git checkout -b feature/my-new-feature
   ```
2. Make your changes and commit them with meaningful commit messages that follow the Conventional Commits specification.
3. Run the linters and tests locally using the provided Taskfile tasks:
   ```bash
   task lint
   task test
   ```
4. Push your changes to the remote repository:
   ```bash
   git push origin feature/my-new-feature
    ```

### Conventional Commits

This project follows the [Conventional Commits](https://www.conventionalcommits.org/en/v1.0.0/) specification for commit messages. This helps to keep a consistent commit history and makes it easier to generate changelogs.

When writing commit messages, please use the following format:

```
<type>([optional scope]): <description>
```

Where `<type>` is one of the following:
- `feat`: A new feature
- `fix`: A bug fix
- `docs`: Documentation changes
- `ci`: Changes to CI configuration files and scripts
- `refactor`: Code changes that neither fix a bug nor add a feature
- `test`: Adding or updating tests
- `chore`: Changes to the build process or auxiliary tools and libraries.

[Commitlint](https://keisukeyamashita.github.io/commitlint-rs/) is used to enforce these rules on commit messages.

We know that sometimes during development, it can be cumbersome to follow these rules strictly. That´s why we only enforce commit message linting on `git push` operations, via the `pre-push` Git hook. This way, you can make as many local commits as you want, and only need to ensure that the commit messages are valid when pushing your changes to the remote repository.

### Pull Request Checklist

When submitting a pull request, please ensure that:
- Your code follows the project's coding style and conventions.
- You have added tests for any new functionality or bug fixes.
- All tests pass.
- Your pull request description clearly explains the changes you have made and the reasons for them.
- Your branch is up to date with the main branch.
- Your commit messages follow the Conventional Commits specification.
- You have linked any related issues in the pull request description.
- You have requested reviews from relevant team members.
- You have addressed any feedback provided during the review process.
- You have squashed your commits into logical units, if necessary.

### Git Commit Hooks

This project uses [Lefthook](https://lefthook.io/) to manage Git commit hooks. Lefthook hooks are automatically installed when you start the Devenv shell.

The following hooks are configured:
- `pre-commit`: Runs code formatting, linters and tests before each commit.
- `pre-push`: Runs commitlint to ensure commit messages follow the conventional commit format.

### Review Process

All pull requests will be reviewed by at least one other team member. The reviewer will check for code quality, adherence to project guidelines, and overall functionality. They may request changes or provide feedback before the pull request can be merged.

### Release Process

Releases are created using GitHub Releases. `draftsman` itself keeps a draft release current on every push to `main` (via `.github/workflows/draftsman.yml`); a maintainer publishes it via the GitHub UI when ready, which triggers GoReleaser to build and attach binaries. See [Architecture](docs/development/architecture.md) and [ADR-0002](docs/adr/0002-continuous-draft-model.md) for how this works.
