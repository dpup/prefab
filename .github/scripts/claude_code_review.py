#!/usr/bin/env python3
"""Perform proactive code review on pull requests."""

import os
import sys
from claude_utils import (
    get_anthropic_client,
    get_github_client,
    get_repo_context,
    get_file_diff,
    get_changed_files,
    create_claude_conversation,
    truncate_text,
    read_file_safe,
)
from rate_limiter import RateLimiter


def perform_code_review():
    """Perform automated code review on a PR."""
    pr_number = int(os.environ.get("PR_NUMBER", "0"))
    base_ref = os.environ.get("BASE_REF", "main")
    head_sha = os.environ.get("HEAD_SHA", "")
    base_sha = os.environ.get("BASE_SHA", "")

    if not pr_number or not head_sha:
        print("Error: Missing PR_NUMBER or HEAD_SHA")
        sys.exit(1)

    print(f"Performing code review for PR #{pr_number}")

    # Get GitHub repo
    gh = get_github_client()
    repo_name = os.environ.get("GITHUB_REPOSITORY")
    repo = gh.get_repo(repo_name)
    pr = repo.get_pull(pr_number)

    # Initialize rate limiter and get config
    rate_limiter = RateLimiter(gh, repo_name)
    review_config = rate_limiter.get_review_config()

    # Check if we should skip this PR
    if review_config.get("skip_draft_prs", True) and pr.draft:
        print("Skipping draft PR")
        sys.exit(0)

    if review_config.get("allow_skip_label", True):
        for label in pr.labels:
            if label.name == "skip-claude-review":
                print("Skipping PR with 'skip-claude-review' label")
                sys.exit(0)

    # Check rate limit
    allowed, reason = rate_limiter.check_code_review()
    if not allowed:
        print(f"Rate limit exceeded: {reason}")
        # Optionally post a comment
        pr.create_issue_comment(
            "⚠️ Daily code review limit reached. A team member will review this PR."
        )
        sys.exit(0)
    print(f"Rate limit check passed: {reason}")

    # Get PR details
    pr_title = pr.title
    pr_body = pr.body or ""

    # Get changed files
    changed_files = get_changed_files(f"origin/{base_ref}", head_sha)
    print(f"Changed files: {len(changed_files)}")

    # Check file count limits
    min_files = review_config.get("min_files_changed", 1)
    max_files = review_config.get("max_files_changed", 50)

    if len(changed_files) < min_files:
        print(f"Skipping PR with {len(changed_files)} files (min: {min_files})")
        sys.exit(0)

    if len(changed_files) > max_files:
        print(f"Skipping PR with {len(changed_files)} files (max: {max_files})")
        pr.create_issue_comment(
            f"⚠️ This PR changes {len(changed_files)} files, which exceeds the automatic review limit of {max_files} files. "
            "A team member will review this manually."
        )
        sys.exit(0)

    # Get full diff
    full_diff = get_file_diff(f"origin/{base_ref}", head_sha)

    # Truncate diff if too large
    diff_truncated = truncate_text(full_diff, 100000)

    # Get repository context
    repo_ctx = get_repo_context()

    # Build system prompt
    system_prompt = f"""You are Claude, an AI code reviewer. Your role is to perform thorough, constructive code reviews.

Repository Guidelines:
{repo_ctx['claude_md']}

Focus on:
1. **Code Quality**: Adherence to project conventions and best practices
2. **Potential Bugs**: Logic errors, edge cases, error handling
3. **Security**: Vulnerabilities, injection risks, data validation
4. **Performance**: Inefficiencies, resource usage
5. **Maintainability**: Code clarity, documentation, test coverage
6. **Go-specific Issues**: For Go code, check proper error handling, goroutine safety, proper use of contexts

Be constructive and specific. Provide code suggestions when appropriate. If the code looks good, acknowledge it briefly."""

    # Build user message
    changed_files_list = "\n".join([f"  - {f}" for f in changed_files[:50]])
    if len(changed_files) > 50:
        changed_files_list += f"\n  ... and {len(changed_files) - 50} more files"

    user_message = f"""Pull Request: #{pr_number}
Title: {pr_title}
Description:
{pr_body}

Changed files ({len(changed_files)}):
{changed_files_list}

Full diff:
```diff
{diff_truncated}
```

Please review these changes and provide constructive feedback. Structure your review with:
1. **Summary**: Brief overview of the changes
2. **Strengths**: What's done well
3. **Concerns**: Issues that should be addressed (if any)
4. **Suggestions**: Improvements and recommendations (if any)
5. **Verdict**: Approve, request changes, or comment

Keep the review focused and actionable."""

    # Get Claude's review
    client = get_anthropic_client()
    review_response = create_claude_conversation(
        client,
        system_prompt,
        user_message,
        max_tokens=16384
    )

    # Post review as a comment
    review_header = "## 🤖 Automated Code Review by Claude\n\n"
    review_footer = "\n\n---\n*This is an automated review. Feel free to ask questions by mentioning @claude in a comment.*"

    full_review = review_header + review_response + review_footer

    pr.create_issue_comment(full_review)
    print("Posted code review")


def main():
    """Main entry point."""
    try:
        perform_code_review()
    except Exception as e:
        print(f"Error performing code review: {e}")
        import traceback
        traceback.print_exc()
        sys.exit(1)


if __name__ == "__main__":
    main()
