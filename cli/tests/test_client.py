import httpx
import pytest

from uptime_cli import client, config


def fake_http(monkeypatch, handler):
    transport = httpx.MockTransport(handler)

    def fake_request(method, url, **kwargs):
        with httpx.Client(transport=transport) as c:
            return c.request(method, url, **kwargs)

    monkeypatch.setattr(httpx, "request", fake_request)


def test_sends_bearer_token(monkeypatch):
    config.save_token("abc123")
    seen = {}

    def handler(request):
        seen["auth"] = request.headers.get("Authorization")
        seen["url"] = str(request.url)
        return httpx.Response(200, json=[{"id": 1}])

    fake_http(monkeypatch, handler)
    result = client.request("GET", "/api/v1/monitors")

    assert result == [{"id": 1}]
    assert seen["auth"] == "Bearer abc123"
    assert seen["url"] == "http://localhost:8080/api/v1/monitors"


def test_not_logged_in_does_not_hit_network(monkeypatch):
    def handler(request):
        raise AssertionError("network must not be called")

    fake_http(monkeypatch, handler)
    with pytest.raises(client.APIError) as exc:
        client.request("GET", "/api/v1/monitors")

    assert exc.value.status == 401


def test_login_skips_token(monkeypatch):
    seen = {}

    def handler(request):
        seen["auth"] = request.headers.get("Authorization")
        return httpx.Response(200, json={"token": "t"})

    fake_http(monkeypatch, handler)
    client.request("POST", "/api/v1/auth/login", json={"email": "e", "password": "p"}, auth=False)

    assert seen["auth"] is None


def test_204_returns_none(monkeypatch):
    config.save_token("t")
    fake_http(monkeypatch, lambda request: httpx.Response(204))

    assert client.request("DELETE", "/api/v1/monitors/1") is None


@pytest.mark.parametrize(
    "response, want_status, want_message",
    [
        (httpx.Response(404, json={"message": "monitor not found"}), 404, "monitor not found"),
        (httpx.Response(403, json={"message": "monitor limit reached for your plan"}), 403, "monitor limit reached for your plan"),
        (httpx.Response(502, text="<html>bad gateway</html>"), 502, "HTTP 502"),
        (httpx.Response(500, json={"unexpected": "shape"}), 500, "HTTP 500"),
        (httpx.Response(400, json={"message": "invalid request body"}), 400, "invalid request body"),
    ],
)
def test_error_responses_become_api_error(monkeypatch, response, want_status, want_message):
    config.save_token("t")
    fake_http(monkeypatch, lambda request: response)

    with pytest.raises(client.APIError) as exc:
        client.request("GET", "/api/v1/monitors/1")

    assert exc.value.status == want_status
    assert exc.value.message == want_message


def test_unreachable_server(monkeypatch):
    config.save_token("t")

    def handler(request):
        raise httpx.ConnectError("connection refused")

    fake_http(monkeypatch, handler)
    with pytest.raises(client.APIError) as exc:
        client.request("GET", "/api/v1/monitors")

    assert exc.value.status == 0
    assert "cannot reach API" in exc.value.message


def test_api_url_from_environment(monkeypatch):
    config.save_token("t")
    monkeypatch.setenv("UPTIME_API_URL", "https://api.example.com/")
    seen = {}

    def handler(request):
        seen["url"] = str(request.url)
        return httpx.Response(200, json={})

    fake_http(monkeypatch, handler)
    client.request("GET", "/api/v1/monitors")

    assert seen["url"] == "https://api.example.com/api/v1/monitors"