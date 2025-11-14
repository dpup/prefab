# Claude AI GitHub Automation

This directory contains GitHub Actions workflows that integrate Claude AI into your development workflow using the official [anthropics/claude-code-action](https://github.com/anthropics/claude-code-action).

## Features

### 1. @claude Mentions (`claude-mention.yml`)

Allows anyone to mention `@claude` in issue and PR comments to get AI assistance.

**Triggers:**
- Issue comments
- Pull request review comments
- Pull request reviews

**Usage:**
```
@claude can you review this implementation?
@claude what's the best way to handle this error case?
@claude help me understand this code
```

**What it does:**
- Responds to questions with context-aware answers
- Provides code review feedback when mentioned on PRs
- Offers implementation suggestions
- Has full access to repository files and context

### 2. Proactive Code Reviews (`claude-review.yml`)

Automatically reviews pull requests when they're opened or updated.

**Triggers:**
- Pull request opened
- Pull request synchronized (new commits pushed)
- Pull request reopened

**What it does:**
- Analyzes code changes in the PR
- Checks for:
  - Code quality and project conventions (using CLAUDE.md)
  - Potential bugs and edge cases
  - Security vulnerabilities
  - Performance issues
  - Go-specific best practices
- Posts comprehensive review comments

**Skips:**
- PRs from `claude/*` branches (avoids reviewing its own work)
- Draft PRs
- PRs with `skip-claude-review` label

### 3. Issue Evaluation & Implementation (`claude-issue-eval.yml`)

Evaluates new issues and attempts to implement reasonable bug fixes or features.

**Triggers:**
- New issue opened (team members only)
- Issue labeled with `claude:evaluate` (external contributors)

**What it does:**
- Evaluates if the issue is clear and actionable
- For implementable issues:
  - Creates an implementation plan
  - Attempts to implement the fix or feature
  - Creates a pull request with the changes
  - Links the PR back to the issue
- For unclear issues:
  - Asks clarifying questions
  - Requests additional context

**Access Controls:**
- Auto-triggers only for repository owners, members, and collaborators
- External contributors must have the `claude:evaluate` label added by a team member

## Setup

### Prerequisites

1. **Anthropic API Key**: You need an API key from Anthropic to use Claude.
2. **GitHub Token**: The workflows use the default `GITHUB_TOKEN` provided by GitHub Actions.

### Configuration

1. **Add the Anthropic API Key as a Secret:**
   - Go to your repository Settings
   - Navigate to Secrets and variables → Actions
   - Click "New repository secret"
   - Name: `ANTHROPIC_API_KEY`
   - Value: Your Anthropic API key
   - Click "Add secret"

2. **Ensure GitHub Actions has proper permissions:**
   - Go to repository Settings
   - Navigate to Actions → General
   - Under "Workflow permissions", select:
     - "Read and write permissions"
     - Check "Allow GitHub Actions to create and approve pull requests"
   - Click "Save"

3. **Enable the workflows:**
   - The workflows are automatically enabled when merged to the main branch
   - You can manually trigger or disable them from the Actions tab

## Workflows Overview

```
.github/
├── workflows/
│   ├── claude-mention.yml         # Handles @claude mentions
│   ├── claude-review.yml          # Proactive code reviews
│   └── claude-issue-eval.yml      # Issue evaluation & implementation
├── claude-config.yml              # Configuration documentation
└── README.md                      # This file
```

## How It Works

All workflows use the official `anthropics/claude-code-action@v1` which:
- Automatically detects context (interactive mode vs automation)
- Has access to repository files and GitHub APIs
- Can create branches, commits, and pull requests
- Uses `CLAUDE.md` for project-specific guidelines
- Provides intelligent code assistance and automation

### @claude Mentions

1. User mentions `@claude` in a comment
2. Workflow triggers and invokes the Claude Code Action
3. Claude analyzes the context (PR diff, issue details, etc.)
4. Posts a helpful response as a comment

### Code Reviews

1. PR is opened or updated
2. Workflow checks if it should skip (draft, claude/* branch, skip label)
3. Claude Code Action reviews the changes
4. Posts comprehensive review feedback

### Issue Implementation

1. New issue is created by a team member (or label is added)
2. Workflow triggers the Claude Code Action
3. Claude evaluates if it's implementable
4. If yes: Creates implementation and submits PR
5. If no: Asks clarifying questions

## Customization

### Adjusting Review Prompts

Edit the `prompt` field in the workflow files to customize what Claude focuses on:

```yaml
- uses: anthropics/claude-code-action@v1
  with:
    anthropic_api_key: ${{ secrets.ANTHROPIC_API_KEY }}
    prompt: |
      Your custom instructions here...
```

### Controlling Access

Access is controlled via GitHub workflow conditionals:

**Code Reviews** - Edit `.github/workflows/claude-review.yml`:
```yaml
if: |
  !startsWith(github.head_ref, 'claude/') &&
  !github.event.pull_request.draft &&
  !contains(github.event.pull_request.labels.*.name, 'skip-claude-review')
```

**Issue Evaluation** - Edit `.github/workflows/claude-issue-eval.yml`:
```yaml
if: |
  (github.event.action == 'opened' &&
   (github.event.issue.author_association == 'OWNER' ||
    github.event.issue.author_association == 'MEMBER' ||
    github.event.issue.author_association == 'COLLABORATOR')) ||
  (github.event.action == 'labeled' &&
   github.event.label.name == 'claude:evaluate')
```

### Additional Options

The Claude Code Action supports many configuration options:

- `trigger_phrase`: Customize the mention syntax (default: `@claude`)
- `use_sticky_comment`: Consolidate PR feedback into a single comment
- `branch_prefix`: Control branch naming convention
- `claude_args`: Pass additional Claude CLI arguments

See the [official documentation](https://github.com/anthropics/claude-code-action) for all options.

## Cost Control

To manage API usage:

- **Use draft PRs** for work-in-progress (skips automatic review)
- **Add `skip-claude-review` label** to skip specific PR reviews
- **Only add `claude:evaluate` label** for issues you want Claude to implement
- **Team members control access** for external contributors

Each workflow run consumes Anthropic API credits:
- @claude mentions: ~4K-10K tokens per interaction
- Code reviews: ~10K-20K tokens per PR
- Issue evaluations: ~8K-20K tokens per issue

Monitor your API usage in the Anthropic console.

## Troubleshooting

### Workflow not triggering

- Check that the workflow files are on the default branch
- Verify GitHub Actions is enabled in repository settings
- Check the Actions tab for error messages

### API key errors

- Verify `ANTHROPIC_API_KEY` is set correctly in repository secrets
- Check the secret name matches exactly (case-sensitive)
- Ensure the API key is valid and has sufficient credits

### Permission errors

- Ensure workflow permissions are set to "Read and write"
- Verify "Allow GitHub Actions to create and approve pull requests" is enabled
- Check that the workflows have the correct `permissions` declarations

### Claude not responding

- Verify the `trigger_phrase` setting (default: `@claude`)
- Check that the comment event is in the workflow's `on:` triggers
- Review workflow run logs in the Actions tab

## Support

For issues with the workflows:
1. Check the Actions tab for detailed logs
2. Review error messages in failed workflow runs
3. Verify all prerequisites are met
4. Consult the [official Claude Code Action docs](https://github.com/anthropics/claude-code-action)
5. Open an issue with details about the problem

## Benefits of Official Action

Using the official `anthropics/claude-code-action` provides:

- ✅ **Maintained by Anthropic** - Regular updates and improvements
- ✅ **Full Claude Code SDK** - Complete feature set
- ✅ **Intelligent mode detection** - Automatically adapts to context
- ✅ **Better tool access** - GitHub APIs, file operations, visual progress
- ✅ **Simpler configuration** - Fewer parameters, cleaner setup
- ✅ **Production ready** - Battle-tested and optimized
