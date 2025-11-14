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
- New issue opened

**What it does:**
- Evaluates if the issue is clear and actionable
- For implementable issues:
  - Creates an implementation plan
  - Creates a new branch (`claude/issue-N-session`)
  - Generates implementation guidance
  - Creates a draft pull request
  - Links the PR back to the issue
- For unclear issues:
  - Asks clarifying questions
  - Requests additional context

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

Monitor your API usage in the Anthropic console.

## Support

For issues with the workflows:
1. Check the Actions tab for detailed logs
2. Review error messages in failed workflow runs
3. Verify all prerequisites are met
4. Open an issue with details about the problem
