import httpx

from uptime_cli import config


class APIError(Exception):
    def __init__(self, status: int, message: str):
        super().__init__(message)
        self.status = status
        self.message = message


def request(method: str, path: str, *, json: dict | None = None,
            params: dict | None = None, auth: bool = True):
    headers = {}
    if auth:
        token = config.load_token()
        if not token:
            raise APIError(401, "not logged in")
        headers["Authorization"] = f"Bearer {token}"

    try:
        resp = httpx.request(
            method,
            config.api_url() + path,
            json=json,
            params=params,
            headers=headers,
            timeout=10.0,
        )
    except httpx.RequestError as exc:
        raise APIError(0, f"cannot reach API at {config.api_url()}: {exc}") from exc

    if resp.status_code >= 400:
        raise APIError(resp.status_code, _extract_message(resp))

    if resp.status_code == 204 or not resp.content:
        return None
    return resp.json()


def _extract_message(resp: httpx.Response) -> str:
    try:
        body = resp.json()
        if isinstance(body, dict) and "message" in body:
            return str(body["message"])
    except ValueError:
        pass
    return f"HTTP {resp.status_code}"