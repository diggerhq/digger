"""
Module for :py:class:`GitHubPR`.
"""
from os import environ

from github import Github


class GitHubPR:
    """
    :py:class:`GitHubPR` represents a pull request on GitHub.
    The pull request is identified by a repository name and a pull request number.
    To access GitHub the class needs a GitHub token.
    All these are input arguments of the class.

    :param repo_name: Full repository name. For example, ``infrahouse/infrahouse-toolkit``.
    :typo repo_name: str
    :param pull_request: Pull request number.
    :type pull_request: int
    :param github_token: GitHub personal access tokens. They are created in
        https://github.com/settings/tokens
    :type github_token: str
    """

    def __init__(self, repo_name: str, pull_request: int, github_token: str = None):
        self._github_token = github_token
        self._repo_name = repo_name
        self._pr_number = pull_request

    @property
    def comments(self):
        """
        An interator with comments in this PR.
        """
        return self.pull_request.get_issue_comments()

    @property
    def github(self):
        """
        GitHub client.
        """
        return Github(login_or_token=self.github_token)

    @property
    def github_token(self):
        """
        GitHub token as passed by the class argument or from the ``GITHUB_TOKEN`` environment variable.
        If the ``GITHUB_TOKEN`` environment variable is not defined the property will return None.
        """
        return self._github_token if self._github_token else environ.get("GITHUB_TOKEN")

    @property
    def repo(self):
        """
        Repository object of the repository name passed in the class argument.
        """
        return self.github.get_repo(self._repo_name)

    @property
    def pull_request(self):
        """
        Pull request object of the repository name passed in the class argument.
        """
        return self.repo.get_pull(self._pr_number)

    def delete_my_comments(self):
        """
        Delete all comments in the pull request.
        """
        for comment in self.comments:
            comment.delete()

    def publish_comment(self, comment: str):
        """Add the given text as a comment in the pull request."""
        self.pull_request.create_issue_comment(comment)

    def changed_files(self):
        return self.pull_request.changed_files
