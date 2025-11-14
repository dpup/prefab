# Claude AI GitHub Automation

This directory contains GitHub Actions workflows and scripts that enable AI-powered automation for this repository.

## Features

### 1. @claude Mentions (`claude-mention.yml`)

Allows team members to mention `@claude` in issue and PR comments to get AI assistance.

**Triggers:**
- Issue comments
- Pull request review comments
- Pull request reviews

**Usage:**
```
@claude can you review this implementation?
@claude what's the best way to handle this error case?
```

**What it does:**
- Responds to questions with context-aware answers
- Provides code review feedback when mentioned on PRs
- Offers implementation suggestions

### 2. Proactive Code Reviews (`claude-review.yml`)

Automatically reviews pull requests when they're opened or updated.

**Triggers:**
- Pull request opened
- Pull request synchronized (new commits pushed)
- Pull request reopened

**What it does:**
- Analyzes code changes in the PR
- Checks for potential bugs, security issues, and performance problems
- Provides constructive feedback following project conventions
- Posts a comprehensive review comment

**Note:** Skips PRs from branches starting with `claude/` to avoid reviewing its own work.

### 3. Issue Evaluation & Implementation (`claude-issue-eval.yml`)

Evaluates new issues and attempts to implement reasonable bug fixes or features.

**Triggers:**
- New issue opened (team members only)
- Issue labeled with `claude:evaluate` (external contributors)

**What it does:**
- Evaluates if the issue is clear and actionable
- For implementable issues from team members:
  - Creates an implementation plan
  - Creates a new branch (`claude/issue-N-session`)
  - Generates implementation guidance
  - Creates a draft pull request
  - Links the PR back to the issue
- For implementable issues from external contributors:
  - Creates an implementation plan (no auto-PR by default)
  - Suggests they can implement following the plan
- For unclear issues:
  - Asks clarifying questions
  - Requests additional context

**Access Controls:**
- Auto-triggers only for repository owners, members, and collaborators
- External contributors must have the `claude:evaluate` label added by a team member
- Rate limits apply to prevent abuse (configurable in `claude-config.yml`)

## Rate Limiting & Cost Controls

To prevent abuse and control API costs, the workflows include several safety features:

### Rate Limits

Default limits (configurable in `.github/claude-config.yml`):
- **Issue Evaluations**: 1 per user per day
- **@claude Mentions**: 1 per user per day
- **Code Reviews**: 1 per day (total)

**Exemptions:**
- Team members (OWNER, MEMBER, COLLABORATOR) are exempt from rate limits by default
- Specific users listed in `exempt_users` bypass all rate limits (e.g., @dpup)

### Code Review Controls

- **Skip draft PRs**: Automatic reviews skip draft PRs (configurable)
- **Skip label**: Add `skip-claude-review` label to skip review
- **File limits**: Skips PRs with < 1 or > 50 files changed (configurable)
- **Skip own work**: Never reviews PRs from `claude/*` branches

### Issue Evaluation Controls

- **Team member only**: Auto-evaluation only for team members by default
- **Label required**: External contributors need `claude:evaluate` label
- **No auto-PR**: External issues get plans but no auto-implementation (configurable)
- **Rate limiting**: Per-user daily limits to prevent spam

### Configuration

Edit `.github/claude-config.yml` to customize limits:

```yaml
rate_limits:
  issues_per_user_per_day: 1
  mentions_per_user_per_day: 1
  reviews_per_day: 1

exempt_team_members: true

# Specific users who bypass all rate limits (GitHub usernames)
exempt_users:
  - dpup

issue_evaluation:
  auto_evaluate_team_members: true
  require_label_for_external: true
  auto_implement_external_issues: false

code_review:
  skip_draft_prs: true
  allow_skip_label: true
  min_files_changed: 1
  max_files_changed: 50
```

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
├── scripts/
│   ├── claude_utils.py            # Shared utilities
│   ├── claude_mention_handler.py  # Mention handling logic
│   ├── claude_code_review.py      # Code review logic
│   └── claude_issue_evaluator.py  # Issue evaluation logic
└── README.md                       # This file
```

## How It Works

### @claude Mentions

1. User mentions `@claude` in a comment
2. Workflow triggers and loads the comment context
3. Script gathers relevant information (PR diff, issue details, etc.)
4. Sends context to Claude API
5. Posts Claude's response as a comment

### Code Reviews

1. PR is opened or updated
2. Workflow fetches the PR diff and metadata
3. Script sends code changes to Claude for review
4. Claude analyzes for:
   - Code quality and conventions
   - Potential bugs and edge cases
   - Security vulnerabilities
   - Performance issues
   - Maintainability concerns
5. Posts comprehensive review as a comment

### Issue Implementation

1. New issue is created
2. Workflow evaluates the issue
3. Claude determines if it's implementable
4. If yes:
   - Creates implementation plan
   - Creates a new branch
   - Generates code changes (saved as implementation plan)
   - Creates draft PR
   - Links PR to issue
5. If no:
   - Posts clarifying questions
   - Waits for author response

## Customization

### Adjusting Prompts

To customize how Claude responds, edit the system prompts in:
- `scripts/claude_mention_handler.py`
- `scripts/claude_code_review.py`
- `scripts/claude_issue_evaluator.py`

### Changing Review Criteria

Edit the system prompt in `claude_code_review.py` to focus on different aspects or add project-specific checks.

### Model Selection

The workflows use `claude-sonnet-4-5-20250929`. To change the model, edit `claude_utils.py`:

```python
model="claude-sonnet-4-5-20250929",  # Change this line
```

## Limitations

- **Token limits**: Very large PRs or diffs may be truncated
- **API rate limits**: Subject to Anthropic API rate limits
- **Implementation**: Issue implementation creates plans but doesn't automatically apply all code changes (requires human review and completion)
- **Context**: Claude has limited context; very complex issues may need human guidance

## Security Considerations

- The `ANTHROPIC_API_KEY` secret must be kept secure
- The workflows run in a sandboxed environment
- Code changes are created as draft PRs for review
- Never merge automatically generated code without review

## Troubleshooting

### Workflow not triggering

- Check that the workflow files are on the default branch
- Verify GitHub Actions is enabled in repository settings
- Check the Actions tab for error messages

### API key errors

- Verify `ANTHROPIC_API_KEY` is set correctly in repository secrets
- Check the secret name matches exactly (case-sensitive)

### Permission errors

- Ensure workflow permissions are set to "Read and write"
- Verify "Allow GitHub Actions to create and approve pull requests" is enabled

## Cost Considerations

Each workflow run consumes Anthropic API credits:
- @claude mentions: ~4K-8K tokens per interaction
- Code reviews: ~10K-20K tokens per PR
- Issue evaluations: ~5K-15K tokens per issue

**Cost Controls:**
- Rate limits prevent excessive usage (see `.github/claude-config.yml`)
- Default limits: 1 issue, 1 mention per user per day, 1 review total per day
- Team members and specific exempt users (like @dpup) bypass limits
- External contributors require manual approval via labels
- Draft PRs and large PRs (>50 files) are skipped

With default settings, maximum daily cost for non-exempt users is approximately:
- 1 code review × 15K tokens = 15K tokens
- 1 mention × 6K tokens per user = varies by user count
- 1 issue evaluation × 10K tokens per user = varies by user count

These tight limits control costs while allowing team members and exempt users unlimited access.

Monitor your API usage in the Anthropic console and adjust limits in `claude-config.yml` as needed.

## Support

For issues with the workflows:
1. Check the Actions tab for detailed logs
2. Review error messages in failed workflow runs
3. Verify all prerequisites are met
4. Open an issue with details about the problem
