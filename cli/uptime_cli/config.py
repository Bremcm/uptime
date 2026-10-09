import json
import os
from pathlib import Path

CONFIG_PATH = Path.home() / ".config" / "uptime" / "config.json"
DEFAULT_API_URL = "http://localhost:8080"


def api_url() -> str:
    return os.environ.get("UPTIME_API_URL", DEFAULT_API_URL).rstrip("/")


def load_token() -> str | None:
    if not CONFIG_PATH.exists():
        return None
    try:
        return json.loads(CONFIG_PATH.read_text()).get("token")
    except (json.JSONDecodeError, OSError):
        return None


def save_token(token: str) -> None:
    CONFIG_PATH.parent.mkdir(parents=True, exist_ok=True)
    CONFIG_PATH.write_text(json.dumps({"token": token}))
    CONFIG_PATH.chmod(0o600)


def clear_token() -> None:
    CONFIG_PATH.unlink(missing_ok=True)