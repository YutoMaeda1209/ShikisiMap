[[日本語 (JA)](/.github/CONTRIBUTING.md) / English (EN)]

# Contribution Guidelines

Please follow these guidelines when contributing, such as reporting bugs, proposing new features, or improving documentation.

## About Issue-Driven Development

This project adopts an "Issue-Driven" development approach. Issue-Driven means that all tasks, including bug fixes, new features, and documentation improvements, are managed as GitHub Issues and work is carried out on a per-Issue basis. This clarifies the purpose, background, and progress of each task, facilitating smooth information sharing and reviews within the team.

- Before starting any work, be sure to create or check an Issue.
- When creating a pull request, please ensure it is linked to an issue. For instructions on linking pull requests to issues, refer to the [official documentation](https://docs.github.com/en/issues/tracking-your-work-with-issues/using-issues/linking-a-pull-request-to-an-issue).

## About Commits

- Each commit should contain only one feature addition or bug fix.
- Commit with no syntax errors.
- The first line of the commit message should follow the format `{verb} {summary}`.
  - `Add user profile page`
  - `Fix login bug`
  - `Update README.md`
- The second line of the commit message should describe the details of the commit.

## Branch Strategy

### Branch Naming Conventions

| Branch Type | Purpose                                       | Example                                   |
| ----------- | --------------------------------------------- | ----------------------------------------- |
| Feature     | Implementing new features                     | `feat/{featureName}`                      |
| Bugfix      | Fixing bugs                                   | `fix/{bugName}`                           |
| Cherry-pick | Applying specific commits from other branches | `cherrypick/{sourceBranch}-{description}` |
| Sandbox     | Experimenting or prototyping                  | `sandbox/{commitId}-{userName}`           |

- `main` branch: The stable branch for production.
- `dev` branch: The integration branch for development. All development branches should be created from `dev` and merged back into `dev` after completion.
- When developing, create a new branch from `dev` according to your purpose.
  - Feature: `feat`<br>
    For implementing new features.
  - Bugfix: `fix`<br>
    For fixing bugs.
  - Cherry-pick: `cherrypick`<br>
    For applying specific commits from other branches.
  - Sandbox: `sandbox`<br>
    For experimenting or prototyping new features.

### Branch Merging Rules

- Create a pull request from your working branch (such as `feat` or `fix`) to the `dev` branch before merging.
- Direct commits or merges to the `main` or `dev` branches are prohibited.
- Always resolve conflicts before merging.

## Contribution Flow

1. Before starting work, create or check an Issue describing the purpose and background.
2. Create a working branch from `dev` and name it according to the Issue (e.g., `feat/featureName`).
3. When your work is complete, create a pull request targeting `dev`, not `main`.
4. Address feedback during review and reflect progress or changes in the Issue as well.
5. After approval by a maintainer, the pull request will be merged into the `dev` branch, the working branch will be deleted, and the pull request will be closed.
