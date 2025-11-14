"""Common utilities for Claude GitHub automation."""

import os
import sys
import subprocess
from typing import Optional, Dict, Any
from anthropic import Anthropic
from github import Github


def get_anthropic_client() -> Anthropic:
    """Get configured Anthropic client."""
    api_key = os.environ.get("ANTHROPIC_API_KEY")
    if not api_key:
        print("Error: ANTHROPIC_API_KEY not set")
        sys.exit(1)
    return Anthropic(api_key=api_key)


def get_github_client() -> Github:
    """Get configured GitHub client."""
    token = os.environ.get("GITHUB_TOKEN")
    if not token:
        print("Error: GITHUB_TOKEN not set")
        sys.exit(1)
    return Github(token)


def run_command(cmd: list[str], cwd: Optional[str] = None) -> tuple[int, str, str]:
    """Run a command and return exit code, stdout, stderr."""
    try:
        result = subprocess.run(
            cmd,
            cwd=cwd,
            capture_output=True,
            text=True,
            timeout=300  # 5 minute timeout
        )
        return result.returncode, result.stdout, result.stderr
    except subprocess.TimeoutExpired:
        return 1, "", "Command timed out after 5 minutes"
    except Exception as e:
        return 1, "", f"Error running command: {e}"


def get_file_diff(base_ref: str, head_ref: str, file_path: Optional[str] = None) -> str:
    """Get diff between two refs, optionally for a specific file."""
    cmd = ["git", "diff", f"{base_ref}...{head_ref}"]
    if file_path:
        cmd.append("--")
        cmd.append(file_path)

    returncode, stdout, stderr = run_command(cmd)
    if returncode != 0:
        return f"Error getting diff: {stderr}"
    return stdout


def get_changed_files(base_ref: str, head_ref: str) -> list[str]:
    """Get list of changed files between two refs."""
    cmd = ["git", "diff", "--name-only", f"{base_ref}...{head_ref}"]
    returncode, stdout, stderr = run_command(cmd)
    if returncode != 0:
        return []
    return [f.strip() for f in stdout.split('\n') if f.strip()]


def create_claude_conversation(
    client: Anthropic,
    system_prompt: str,
    user_message: str,
    max_tokens: int = 4096
) -> str:
    """Create a conversation with Claude and return the response."""
    try:
        message = client.messages.create(
            model="claude-sonnet-4-5-20250929",
            max_tokens=max_tokens,
            system=system_prompt,
            messages=[
                {"role": "user", "content": user_message}
            ]
        )
        return message.content[0].text
    except Exception as e:
        print(f"Error calling Claude API: {e}")
        return f"Error: Unable to get response from Claude - {e}"


def get_repo_context() -> Dict[str, Any]:
    """Get repository context information."""
    # Read CLAUDE.md if it exists
    claude_md = ""
    if os.path.exists("CLAUDE.md"):
        with open("CLAUDE.md", "r") as f:
            claude_md = f.read()

    # Read README.md if it exists
    readme = ""
    if os.path.exists("README.md"):
        with open("README.md", "r") as f:
            readme = f.read()

    return {
        "claude_md": claude_md,
        "readme": readme,
    }


def read_file_safe(file_path: str) -> Optional[str]:
    """Safely read a file, returning None if it doesn't exist or can't be read."""
    try:
        with open(file_path, "r") as f:
            return f.read()
    except Exception:
        return None


def truncate_text(text: str, max_length: int = 10000) -> str:
    """Truncate text to max length with ellipsis."""
    if len(text) <= max_length:
        return text
    return text[:max_length] + "\n\n... [truncated]"
