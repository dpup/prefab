#!/usr/bin/env python3
"""Handle @claude mentions in issues and pull requests."""

import os
import json
import sys
from claude_utils import (
    get_anthropic_client,
    get_github_client,
    get_repo_context,
    get_file_diff,
    create_claude_conversation,
    truncate_text,
)
from rate_limiter import RateLimiter


def handle_pr_mention(repo, pr_number: int, comment_body: str, comment_user: str, comment_author_association: str):
    """Handle @claude mention in a pull request."""
    print(f"Handling PR mention in #{pr_number}")

    pr = repo.get_pull(pr_number)

    # Get PR context
    pr_title = pr.title
    pr_body = pr.body or ""
    base_ref = pr.base.ref
    head_sha = pr.head.sha
    base_sha = pr.base.sha

    # Get diff
    diff = get_file_diff(f"origin/{base_ref}", head_sha)
    diff_truncated = truncate_text(diff, 50000)

    # Get repository context
    repo_ctx = get_repo_context()

    # Build system prompt
    system_prompt = f"""You are Claude, an AI assistant helping with code review and pull request discussions.

Repository Guidelines:
{repo_ctx['claude_md']}

You are reviewing a pull request. Provide helpful, constructive feedback."""

    # Build user message
    user_message = f"""Pull Request: #{pr_number}
Title: {pr_title}
Description: {pr_body}

Changes (diff):
```diff
{diff_truncated}
```

User @{comment_user} mentioned you with this comment:
{comment_body}

Please provide a helpful response. If they're asking for a code review, analyze the changes and provide constructive feedback. If they're asking questions, answer them based on the code changes and context."""

    # Get Claude's response
    client = get_anthropic_client()
    response = create_claude_conversation(client, system_prompt, user_message, max_tokens=8192)

    # Post response as a comment
    pr.create_issue_comment(f"@{comment_user}\n\n{response}")
    print("Posted response to PR")


def handle_issue_mention(repo, issue_number: int, comment_body: str, comment_user: str, comment_author_association: str):
    """Handle @claude mention in an issue."""
    print(f"Handling issue mention in #{issue_number}")

    issue = repo.get_issue(issue_number)

    # Get issue context
    issue_title = issue.title
    issue_body = issue.body or ""

    # Get repository context
    repo_ctx = get_repo_context()

    # Build system prompt
    system_prompt = f"""You are Claude, an AI assistant helping with GitHub issues and project discussions.

Repository Guidelines:
{repo_ctx['claude_md']}

Repository Overview:
{truncate_text(repo_ctx['readme'], 5000)}

You are helping with an issue discussion. Provide helpful, actionable advice."""

    # Build user message
    user_message = f"""Issue: #{issue_number}
Title: {issue_title}
Description: {issue_body}

User @{comment_user} mentioned you with this comment:
{comment_body}

Please provide a helpful response. Answer their questions, provide guidance, or suggest solutions based on the issue context."""

    # Get Claude's response
    client = get_anthropic_client()
    response = create_claude_conversation(client, system_prompt, user_message, max_tokens=8192)

    # Post response as a comment
    issue.create_comment(f"@{comment_user}\n\n{response}")
    print("Posted response to issue")


def main():
    """Main entry point."""
    # Load event data
    event_path = os.environ.get("GITHUB_EVENT_PATH")
    if not event_path:
        print("Error: GITHUB_EVENT_PATH not set")
        sys.exit(1)

    with open(event_path, "r") as f:
        event = json.load(f)

    # Get event type
    event_name = os.environ.get("GITHUB_EVENT_NAME")

    # Get GitHub repo
    gh = get_github_client()
    repo_name = os.environ.get("GITHUB_REPOSITORY")
    repo = gh.get_repo(repo_name)

    # Initialize rate limiter
    rate_limiter = RateLimiter(gh, repo_name)

    # Extract comment info based on event type
    if event_name == "issue_comment":
        comment = event.get("comment", {})
        comment_body = comment.get("body", "")
        comment_user = comment.get("user", {}).get("login", "unknown")
        comment_author_association = comment.get("author_association", "NONE")

        # Check rate limit
        allowed, reason = rate_limiter.check_mention_response(comment_user, comment_author_association)
        if not allowed:
            print(f"Rate limit exceeded: {reason}")
            sys.exit(0)
        print(f"Rate limit check passed: {reason}")

        # Check if this is a PR or issue
        issue = event.get("issue", {})
        if "pull_request" in issue:
            # This is a PR comment
            pr_number = issue.get("number")
            handle_pr_mention(repo, pr_number, comment_body, comment_user, comment_author_association)
        else:
            # This is an issue comment
            issue_number = issue.get("number")
            handle_issue_mention(repo, issue_number, comment_body, comment_user, comment_author_association)

    elif event_name == "pull_request_review":
        review = event.get("review", {})
        comment_body = review.get("body", "")
        comment_user = review.get("user", {}).get("login", "unknown")
        comment_author_association = review.get("author_association", "NONE")

        # Check rate limit
        allowed, reason = rate_limiter.check_mention_response(comment_user, comment_author_association)
        if not allowed:
            print(f"Rate limit exceeded: {reason}")
            sys.exit(0)
        print(f"Rate limit check passed: {reason}")

        pr = event.get("pull_request", {})
        pr_number = pr.get("number")
        handle_pr_mention(repo, pr_number, comment_body, comment_user, comment_author_association)

    elif event_name == "pull_request_review_comment":
        comment = event.get("comment", {})
        comment_body = comment.get("body", "")
        comment_user = comment.get("user", {}).get("login", "unknown")
        comment_author_association = comment.get("author_association", "NONE")

        # Check rate limit
        allowed, reason = rate_limiter.check_mention_response(comment_user, comment_author_association)
        if not allowed:
            print(f"Rate limit exceeded: {reason}")
            sys.exit(0)
        print(f"Rate limit check passed: {reason}")

        pr = event.get("pull_request", {})
        pr_number = pr.get("number")
        handle_pr_mention(repo, pr_number, comment_body, comment_user, comment_author_association)

    else:
        print(f"Unsupported event type: {event_name}")
        sys.exit(1)


if __name__ == "__main__":
    main()
