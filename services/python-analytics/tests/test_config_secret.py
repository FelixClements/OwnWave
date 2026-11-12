import pytest

from config import analytics_secret_problem


@pytest.mark.parametrize(
    "secret",
    ["", "   ", "local-analytics-secret-change-me", "replace-me-generate-analytics-secret-32ch", "CHANGEME", "short"],
)
def test_rejects_unusable_secrets(secret):
    assert analytics_secret_problem(secret)


def test_accepts_generated_secret():
    assert analytics_secret_problem("0123456789abcdef0123456789abcdef") is None
