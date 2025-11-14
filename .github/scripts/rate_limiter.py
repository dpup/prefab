"""Rate limiting for Claude automation workflows."""

import os
import yaml
from datetime import datetime, timedelta
from typing import Dict, Optional
from github import Github


class RateLimiter:
    """Simple rate limiter using GitHub issue labels and comments for tracking."""

    def __init__(self, gh: Github, repo_name: str):
        self.gh = gh
        self.repo = gh.get_repo(repo_name)
        self.config = self._load_config()

    def _load_config(self) -> Dict:
        """Load configuration from claude-config.yml."""
        config_path = ".github/claude-config.yml"
        try:
            with open(config_path, "r") as f:
                return yaml.safe_load(f) or {}
        except FileNotFoundError:
            print(f"Warning: {config_path} not found, using defaults")
            return {}

    def _is_team_member(self, author_association: str) -> bool:
        """Check if user is a team member."""
        return author_association in ["OWNER", "MEMBER", "COLLABORATOR"]

    def _get_rate_limit_issue(self) -> Optional[any]:
        """Get or create the rate limit tracking issue."""
        # Look for existing rate limit tracking issue
        issues = self.repo.get_issues(
            state="open",
            labels=["claude:rate-limit-tracker"]
        )

        for issue in issues:
            return issue

        # Create if doesn't exist
        issue = self.repo.create_issue(
            title="[Internal] Claude Rate Limit Tracker",
            body="This issue tracks rate limits for Claude automation. Do not close.",
            labels=["claude:rate-limit-tracker"]
        )
        return issue

    def _get_today_key(self) -> str:
        """Get today's date key for tracking."""
        return datetime.utcnow().strftime("%Y-%m-%d")

    def _parse_usage_from_comments(self, issue) -> Dict[str, Dict[str, int]]:
        """Parse usage data from issue comments."""
        usage = {}
        today = self._get_today_key()

        for comment in issue.get_comments():
            # Parse comments in format: "DATE|USER|TYPE|COUNT"
            try:
                body = comment.body.strip()
                if not body.startswith("USAGE:"):
                    continue

                _, data = body.split(":", 1)
                date, user, op_type, count = data.strip().split("|")

                # Only consider today's data
                if date != today:
                    continue

                if user not in usage:
                    usage[user] = {}
                usage[user][op_type] = int(count)
            except (ValueError, IndexError):
                continue

        return usage

    def _increment_usage(self, tracker_issue, user: str, operation: str):
        """Increment usage counter for user and operation."""
        usage = self._parse_usage_from_comments(tracker_issue)
        today = self._get_today_key()

        current = usage.get(user, {}).get(operation, 0)
        new_count = current + 1

        # Post new usage comment
        tracker_issue.create_comment(f"USAGE: {today}|{user}|{operation}|{new_count}")

    def check_issue_evaluation(self, user: str, author_association: str) -> tuple[bool, str]:
        """Check if issue evaluation is allowed for this user."""
        config = self.config.get("rate_limits", {})

        # Check if team members are exempt
        if self.config.get("exempt_team_members", True) and self._is_team_member(author_association):
            return True, "Team member - no limits"

        # Check rate limit
        limit = config.get("issues_per_user_per_day", 3)
        if limit == 0:  # Unlimited
            return True, "No limit configured"

        # Get current usage
        tracker = self._get_rate_limit_issue()
        usage = self._parse_usage_from_comments(tracker)
        current = usage.get(user, {}).get("issue_eval", 0)

        if current >= limit:
            return False, f"Rate limit exceeded: {current}/{limit} issue evaluations today"

        # Increment usage
        self._increment_usage(tracker, user, "issue_eval")
        return True, f"Usage: {current + 1}/{limit}"

    def check_mention_response(self, user: str, author_association: str) -> tuple[bool, str]:
        """Check if mention response is allowed for this user."""
        config = self.config.get("rate_limits", {})

        # Check if team members are exempt
        if self.config.get("exempt_team_members", True) and self._is_team_member(author_association):
            return True, "Team member - no limits"

        # Check rate limit
        limit = config.get("mentions_per_user_per_day", 10)
        if limit == 0:  # Unlimited
            return True, "No limit configured"

        # Get current usage
        tracker = self._get_rate_limit_issue()
        usage = self._parse_usage_from_comments(tracker)
        current = usage.get(user, {}).get("mention", 0)

        if current >= limit:
            return False, f"Rate limit exceeded: {current}/{limit} @claude mentions today"

        # Increment usage
        self._increment_usage(tracker, user, "mention")
        return True, f"Usage: {current + 1}/{limit}"

    def check_code_review(self) -> tuple[bool, str]:
        """Check if code review is allowed (global daily limit)."""
        config = self.config.get("rate_limits", {})
        limit = config.get("reviews_per_day", 20)

        if limit == 0:  # Unlimited
            return True, "No limit configured"

        # Get current usage
        tracker = self._get_rate_limit_issue()
        usage = self._parse_usage_from_comments(tracker)

        # Sum all users' review counts
        total = 0
        for user_usage in usage.values():
            total += user_usage.get("review", 0)

        if total >= limit:
            return False, f"Daily review limit exceeded: {total}/{limit} reviews today"

        # Increment usage (use "system" as user for global limit)
        self._increment_usage(tracker, "system", "review")
        return True, f"Usage: {total + 1}/{limit}"

    def should_auto_implement(self, author_association: str) -> bool:
        """Check if we should auto-implement for this user."""
        config = self.config.get("issue_evaluation", {})

        if self._is_team_member(author_association):
            return config.get("auto_implement_team_member_issues", True)
        else:
            return config.get("auto_implement_external_issues", False)

    def get_review_config(self) -> Dict:
        """Get code review configuration."""
        return self.config.get("code_review", {})
