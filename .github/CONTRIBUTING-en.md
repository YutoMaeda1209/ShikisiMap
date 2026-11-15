[[日本語 (JA)](/.github/CONTRIBUTING.md) / English (EN)]

# Contribution Guidelines

Please follow these guidelines when contributing, such as reporting bugs, proposing new features, or improving documentation.

## About Issue-Driven Development

This project adopts an "Issue-Driven" development approach. Issue-Driven means that all tasks, including bug fixes, new features, and documentation improvements, are managed as GitHub Issues and work is carried out on a per-Issue basis. This clarifies the purpose, background, and progress of each task, facilitating smooth information sharing and reviews within the team.

- Before starting any work, be sure to create or check an Issue.
- Always link your branch to an Issue. (e.g., `feat/123-newFeature`)
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
| Feature     | Implementing new features                     | `feat/{IssueNum}-{featureName}`           |
| Bugfix      | Fixing bugs                                   | `fix/{IssueNum}-{bugName}`                |
| Cherry-pick | Applying specific commits from other branches | `cherrypick/{sourceBranch}-{description}` |
| Sandbox     | Experimenting or prototyping                  | `sandbox/{commitId}-{userName}`           |

- `main` branch: The stable branch for production.
- When developing, create a new branch from `main` according to your purpose.
  - Feature: `feat`<br>
    For implementing new features.
  - Bugfix: `fix`<br>
    For fixing bugs.
  - Cherry-pick: `cherrypick`<br>
    For applying specific commits from other branches.
  - Sandbox: `sandbox`<br>
    For experimenting or prototyping new features.

### Branch Merging Rules

- Create a pull request from your working branch (such as `feat` or `fix`) to the `main` branch before merging.
- Direct commits or merges to the `main` branch are prohibited.
- Always resolve conflicts before merging.

## About AI Usage

You may use AI tools (e.g., code generation, writing assistance) when contributing, but only if you can take full responsibility for the result.

- Review, understand, and verify all AI-generated content (code, documentation, Issues, pull requests) before submitting it.
- Do not submit content that you cannot explain or maintain yourself.
- Make sure AI-generated content does not violate licenses or include confidential information.
- You, not the AI, are responsible for everything you submit.

## Contribution Flow

1. Before starting work, create or check an Issue describing the purpose and background.
2. Create a working branch from `main` and name it according to the Issue (e.g., `feat/123-featureName`).
3. When your work is complete, create a pull request targeting `main`.
4. Address feedback during review and reflect progress or changes in the Issue as well.
5. After approval by a maintainer, the pull request will be merged into the `main` branch, the working branch will be deleted, and the pull request will be closed.

## Adding Spots

Follow these guidelines when adding a spot to ShikisiMap, the fan-made pilgrimage map for Shikishima Tetora.

### Purpose

- Provide useful information to users
- Avoid duplicate or incorrect data and keep it easy to maintain

### Eligibility

- The spot complies with the [Shikishima Tetora / Shikishima Heavy Industries Fan Work Guidelines](https://tetratex01.my.canva.site/guideline)
- Shikishima Tetora actually visited the place in content related to Shikishima Tetora
- Shikishima Tetora mentions it sufficiently
- It is mentioned in content that anyone can access without restriction (not paid content, etc.)

### Quality Standards

Make sure a new spot meets the following quality standards.

- Accuracy: Take location data from reliable sources such as Google Maps or OpenStreetMap, and confirm closure information with multiple sources
- Usefulness: The spot is sufficiently mentioned in Shikishima Tetora's content and was actually visited
- Legality: The spot is not a restricted area or private property at the time it is added

### Choosing the Cited Video

If the same spot appears in several videos, choose the cited video by these criteria.

- The video in which Shikishima Tetora first visited the spot
- The video in which Shikishima Tetora mentions the spot in the most detail

### Format

Spots are added to `src/frontend/src/data.json` in [GeoJSON](https://datatracker.ietf.org/doc/html/rfc7946) format. Follow the field list below.

- `type`: string - always `Feature`
- `properties`
  - `name`: string - spot name
  - `address`: string - exact address of the spot
  - `isClosed`: boolean - `true` if the spot is currently closed
  - `youtubeId`: string - ID of the YouTube video in which the spot appears
  - `timestamp`: string - timestamp (seconds) in the YouTube video at which the spot appears
- `geometry`
  - `type`: string - always `Point`
  - `coordinates`: array[number] - coordinates of `address` ([longitude, latitude])

### Review Procedure

1. Propose the spot to add in an Issue
1. Add the spot to `src/frontend/src/data.json`
1. Create a pull request following the [Contribution Flow](#contribution-flow)
1. At least one reviewer checks the eligibility, format and information
1. A reviewer runs the script to assign an ID
1. Merge after approval
