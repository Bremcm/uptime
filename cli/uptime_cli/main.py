from typing import Optional

import typer

from uptime_cli import client, config

app = typer.Typer(help="Command-line client for the Uptime monitoring API.", no_args_is_help=True)
monitors = typer.Typer(help="Manage monitors.", no_args_is_help=True)
notify = typer.Typer(help="Configure notification channels.", no_args_is_help=True)
app.add_typer(monitors, name="monitors")
app.add_typer(notify, name="notify")


def call(method: str, path: str, **kwargs):
    try:
        return client.request(method, path, **kwargs)
    except client.APIError as err:
        if err.status == 401 and kwargs.get("auth", True):
            typer.secho("Not logged in or token expired. Run: uptime login", fg=typer.colors.RED, err=True)
        else:
            typer.secho(f"Error: {err.message}", fg=typer.colors.RED, err=True)
        raise typer.Exit(code=1)


def print_monitor(m: dict) -> None:
    state = "enabled" if m["enabled"] else "disabled"
    typer.echo(f"#{m['id']}  {m['name']}  [{state}]")
    typer.echo(f"  url:      {m['url']}")
    typer.echo(f"  interval: {m['interval_seconds']}s")
    typer.echo(f"  created:  {m['created_at']}")


@app.command()
def login(email: str = typer.Option(..., prompt=True, help="Account email")):
    """Log in and store the token locally."""
    password = typer.prompt("Password", hide_input=True)
    data = call("POST", "/api/v1/auth/login", json={"email": email, "password": password}, auth=False)
    config.save_token(data["token"])
    typer.echo("Logged in.")


@app.command()
def logout():
    """Forget the stored token."""
    config.clear_token()
    typer.echo("Logged out.")


@monitors.command("list")
def list_monitors():
    """List your monitors."""
    rows = call("GET", "/api/v1/monitors")
    if not rows:
        typer.echo("No monitors yet.")
        return
    typer.echo(f"{'ID':<5}{'NAME':<24}{'INTERVAL':<10}{'STATE':<10}URL")
    for m in rows:
        state = "enabled" if m["enabled"] else "disabled"
        typer.echo(f"{m['id']:<5}{m['name'][:22]:<24}{str(m['interval_seconds']) + 's':<10}{state:<10}{m['url']}")


@monitors.command("add")
def add_monitor(
    name: str,
    url: str,
    interval: int = typer.Option(300, help="Check interval in seconds"),
):
    """Create a monitor."""
    m = call("POST", "/api/v1/monitors", json={"name": name, "url": url, "interval_seconds": interval})
    print_monitor(m)


@monitors.command("show")
def show_monitor(monitor_id: int):
    """Show one monitor."""
    print_monitor(call("GET", f"/api/v1/monitors/{monitor_id}"))


@monitors.command("update")
def update_monitor(
    monitor_id: int,
    name: Optional[str] = typer.Option(None, help="New name"),
    url: Optional[str] = typer.Option(None, help="New URL"),
    interval: Optional[int] = typer.Option(None, help="New interval in seconds"),
    enabled: Optional[bool] = typer.Option(None, "--enable/--disable", help="Turn the monitor on or off"),
):
    """Change some fields of a monitor."""
    payload = {}
    if name is not None:
        payload["name"] = name
    if url is not None:
        payload["url"] = url
    if interval is not None:
        payload["interval_seconds"] = interval
    if enabled is not None:
        payload["enabled"] = enabled
    if not payload:
        typer.secho("Nothing to update. Pass at least one option.", fg=typer.colors.RED, err=True)
        raise typer.Exit(code=1)
    print_monitor(call("PATCH", f"/api/v1/monitors/{monitor_id}", json=payload))


@monitors.command("delete")
def delete_monitor(
    monitor_id: int,
    yes: bool = typer.Option(False, "--yes", "-y", help="Skip the confirmation"),
):
    """Delete a monitor."""
    if not yes:
        typer.confirm(f"Delete monitor {monitor_id}?", abort=True)
    call("DELETE", f"/api/v1/monitors/{monitor_id}")
    typer.echo("Deleted.")


@app.command()
def stats(
    monitor_id: int,
    from_: Optional[str] = typer.Option(None, "--from", help="RFC3339 start, e.g. 2026-10-09T00:00:00Z"),
    to: Optional[str] = typer.Option(None, "--to", help="RFC3339 end"),
):
    """Show hourly latency and uptime for a monitor."""
    params = {}
    if from_:
        params["from"] = from_
    if to:
        params["to"] = to
    points = call("GET", f"/api/v1/monitors/{monitor_id}/stats", params=params)
    if not points:
        typer.echo("No data for this period.")
        return
    typer.echo(f"{'HOUR':<22}{'AVG LATENCY':<14}UPTIME")
    for p in points:
        typer.echo(f"{p['hour']:<22}{p['avg_latency']:<14.1f}{p['uptime_pct']:.1f}%")


@notify.command("telegram")
def notify_telegram(chat_id: str):
    """Send incident alerts to a Telegram chat."""
    call("PUT", "/api/v1/me/telegram", json={"chat_id": chat_id})
    typer.echo("Telegram channel saved.")


@notify.command("email")
def notify_email(address: str):
    """Send incident alerts to an email address."""
    call("PUT", "/api/v1/me/notification-email", json={"email": address})
    typer.echo("Email channel saved.")


@notify.command("webhook")
def notify_webhook(url: str):
    """POST incident alerts to a URL."""
    call("PUT", "/api/v1/me/webhook", json={"url": url})
    typer.echo("Webhook saved.")