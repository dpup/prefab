#!/usr/bin/env python3
"""Evaluate new issues and attempt implementation or ask for clarification."""

import os
import sys
import subprocess
from claude_utils import (
    get_anthropic_client,
    get_github_client,
    get_repo_context,
    create_claude_conversation,
    truncate_text,
    run_command,
)
from rate_limiter import RateLimiter


def evaluate_issue():
    """Evaluate a new issue and decide whether to implement or ask for clarification."""
    issue_number = int(os.environ.get("ISSUE_NUMBER", "0"))
    session_id = os.environ.get("GITHUB_SESSION_ID", "")

    if not issue_number:
        print("Error: Missing ISSUE_NUMBER")
        sys.exit(1)

    print(f"Evaluating issue #{issue_number}")

    # Get GitHub repo
    gh = get_github_client()
    repo_name = os.environ.get("GITHUB_REPOSITORY")
    repo = gh.get_repo(repo_name)
    issue = repo.get_issue(issue_number)

    # Get issue details
    issue_title = issue.title
    issue_body = issue.body or ""
    issue_author = issue.user.login
    author_association = issue.author_association

    # Check rate limits
    rate_limiter = RateLimiter(gh, repo_name)
    allowed, reason = rate_limiter.check_issue_evaluation(issue_author, author_association)

    if not allowed:
        print(f"Rate limit check failed: {reason}")
        issue.create_comment(
            f"@{issue_author} Thank you for your issue! I've reached my daily limit for "
            "issue evaluations. A team member will review this soon, or you can try again tomorrow.\n\n"
            f"_{reason}_"
        )
        sys.exit(0)

    print(f"Rate limit check passed: {reason}")

    # Get repository context
    repo_ctx = get_repo_context()

    # Get repository structure (limited to avoid token overload)
    returncode, tree_output, _ = run_command(
        ["find", ".", "-type", "f", "-name", "*.go", "|", "head", "-100"],
        cwd="."
    )
    if returncode != 0:
        returncode, tree_output, _ = run_command(["ls", "-R"])

    # Build evaluation prompt
    system_prompt = f"""You are Claude, an AI assistant evaluating GitHub issues to determine if they should be implemented.

Repository Guidelines:
{repo_ctx['claude_md']}

Repository Overview:
{truncate_text(repo_ctx['readme'], 5000)}

Your task is to evaluate whether this issue is:
1. Clear and actionable
2. A reasonable bug fix or feature request
3. Something you can implement without significant ambiguity

Respond with a JSON object containing:
- "implementable": boolean - whether you can implement this
- "reasoning": string - brief explanation of your decision
- "questions": array of strings - questions to ask if not implementable
- "implementation_plan": string - brief plan if implementable (empty if not)

Be conservative - only mark as implementable if you're confident you understand the requirement and can implement it correctly."""

    user_message = f"""Issue #{issue_number}
Submitted by: @{issue_author}
Title: {issue_title}

Description:
{issue_body}

Repository structure (partial):
{truncate_text(tree_output, 3000)}

Evaluate this issue and respond with your assessment in JSON format."""

    # Get Claude's evaluation
    client = get_anthropic_client()
    evaluation_response = create_claude_conversation(
        client,
        system_prompt,
        user_message,
        max_tokens=4096
    )

    print(f"Evaluation response: {evaluation_response}")

    # Parse the response
    try:
        import json
        # Extract JSON from markdown code blocks if present
        if "```json" in evaluation_response:
            json_start = evaluation_response.find("```json") + 7
            json_end = evaluation_response.find("```", json_start)
            json_str = evaluation_response[json_start:json_end].strip()
        elif "```" in evaluation_response:
            json_start = evaluation_response.find("```") + 3
            json_end = evaluation_response.find("```", json_start)
            json_str = evaluation_response[json_start:json_end].strip()
        else:
            json_str = evaluation_response.strip()

        evaluation = json.loads(json_str)
    except json.JSONDecodeError as e:
        print(f"Error parsing evaluation response: {e}")
        # Fall back to asking for clarification
        evaluation = {
            "implementable": False,
            "reasoning": "Unable to parse evaluation",
            "questions": ["Could you provide more details about this request?"],
            "implementation_plan": ""
        }

    # Post initial response
    if evaluation.get("implementable", False):
        # Attempt implementation
        implementation_plan = evaluation.get("implementation_plan", "")

        # Check if we should auto-implement for this user
        should_implement = rate_limiter.should_auto_implement(author_association)

        if should_implement:
            comment = f"""## 🤖 Issue Evaluation

Thanks for opening this issue, @{issue_author}!

**Assessment**: This looks like a reasonable request that I can help implement.

**Plan**:
{implementation_plan}

I'll create a pull request with an implementation. Please review it and let me know if any adjustments are needed.

---
*Automated evaluation by Claude*"""

            issue.create_comment(comment)
            print("Posted implementation plan comment")

            # Attempt implementation using extended interaction
            success = attempt_implementation(
                client, repo, issue, issue_number, session_id, repo_ctx
            )
        else:
            # Just post the plan without implementing
            comment = f"""## 🤖 Issue Evaluation

Thanks for opening this issue, @{issue_author}!

**Assessment**: This looks like a reasonable request that could be implemented.

**Plan**:
{implementation_plan}

A team member will review this issue and may create an implementation. If you're interested in contributing, feel free to follow this plan and submit a pull request!

---
*Automated evaluation by Claude*"""

            issue.create_comment(comment)
            print("Posted evaluation without auto-implementation")

        if not success:
            # Post follow-up if implementation failed
            issue.create_comment(
                f"@{issue_author} I encountered some difficulties implementing this. "
                "Could you provide more details or clarify the requirements?"
            )

    else:
        # Ask for clarification
        reasoning = evaluation.get("reasoning", "")
        questions = evaluation.get("questions", [])
        questions_list = "\n".join([f"{i+1}. {q}" for i, q in enumerate(questions)])

        comment = f"""## 🤖 Issue Evaluation

Thanks for opening this issue, @{issue_author}!

**Assessment**: I need some clarification before implementing this.

**Reasoning**: {reasoning}

**Questions**:
{questions_list}

Once you provide these details, I'll be happy to help implement this!

---
*Automated evaluation by Claude*"""

        issue.create_comment(comment)
        print("Posted clarification request")


def attempt_implementation(
    client, repo, issue, issue_number: int, session_id: str, repo_ctx: dict
) -> bool:
    """Attempt to implement the issue by creating code changes and a PR."""
    try:
        # Create a branch name
        # Use session_id to ensure uniqueness and match required pattern
        branch_name = f"claude/issue-{issue_number}-{session_id}"

        print(f"Creating branch: {branch_name}")

        # Create and checkout new branch
        run_command(["git", "checkout", "-b", branch_name])

        # Build implementation prompt
        system_prompt = f"""You are Claude, an AI software engineer implementing GitHub issues.

Repository Guidelines:
{repo_ctx['claude_md']}

You need to implement the requested changes. Provide specific file modifications as a series of instructions.

For each file change, specify:
1. The file path
2. Whether it's a new file or modification
3. The complete content (for new files) or specific changes (for modifications)

Format your response as a series of file operations in this format:

FILE: path/to/file.go
ACTION: create|modify
CONTENT:
```
file content here
```

Be thorough but conservative. Only make necessary changes. Include tests if appropriate."""

        issue_title = issue.title
        issue_body = issue.body or ""

        user_message = f"""Issue #{issue_number}: {issue_title}

{issue_body}

Please implement this change following the repository's conventions. Provide the file changes needed."""

        # Get implementation instructions
        impl_response = create_claude_conversation(
            client,
            system_prompt,
            user_message,
            max_tokens=16384
        )

        print(f"Implementation response received (length: {len(impl_response)})")

        # For now, post implementation guidance as a comment and create a draft PR
        # A more sophisticated implementation would parse the response and apply changes
        # This would require more complex logic to handle file operations safely

        # Create a simple change to demonstrate the workflow
        # In production, you'd parse impl_response and apply the changes
        implementation_file = ".github/claude-implementation.txt"
        with open(implementation_file, "w") as f:
            f.write(f"Implementation plan for issue #{issue_number}\n\n")
            f.write(impl_response)

        # Commit the changes
        run_command(["git", "add", implementation_file])
        run_command([
            "git", "commit", "-m",
            f"Add implementation plan for issue #{issue_number}\n\nGenerated by Claude AI"
        ])

        # Push the branch
        repo_url = f"https://x-access-token:{os.environ['GITHUB_TOKEN']}@github.com/{os.environ['GITHUB_REPOSITORY']}.git"
        push_result = run_command(["git", "push", "-u", repo_url, branch_name])

        if push_result[0] != 0:
            print(f"Push failed: {push_result[2]}")
            return False

        print("Pushed branch successfully")

        # Create PR
        pr_body = f"""## Automated Implementation for Issue #{issue_number}

Closes #{issue_number}

## Implementation Details

{impl_response[:5000]}

{'... [truncated]' if len(impl_response) > 5000 else ''}

---

**Note**: This is an automated implementation by Claude AI. Please review carefully and test before merging.

The implementation plan has been saved to `{implementation_file}`. To complete the implementation, the specific code changes described in that file should be applied.

Feel free to:
- Request changes or adjustments
- Mention @claude in comments for questions
- Close this PR if it doesn't meet your needs
"""

        pr = repo.create_pull(
            title=f"Fix #{issue_number}: {issue.title}",
            body=pr_body,
            head=branch_name,
            base=repo.default_branch,
            draft=True  # Create as draft since it's automated
        )

        print(f"Created PR #{pr.number}")

        # Add comment to issue linking to PR
        issue.create_comment(
            f"I've created a draft pull request #{pr.number} with an implementation plan. "
            "Please review and let me know if you'd like any changes!"
        )

        return True

    except Exception as e:
        print(f"Error during implementation: {e}")
        import traceback
        traceback.print_exc()
        return False


def main():
    """Main entry point."""
    try:
        evaluate_issue()
    except Exception as e:
        print(f"Error evaluating issue: {e}")
        import traceback
        traceback.print_exc()
        sys.exit(1)


if __name__ == "__main__":
    main()
