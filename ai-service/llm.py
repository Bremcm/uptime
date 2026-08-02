import requests
from datetime import datetime


OLLAMA_URL = "http://localhost:11434/api/generate"
MODEL = "llama3.2"


def explain(prompt):
    response = requests.post(
        OLLAMA_URL,
        json={"model": MODEL, "prompt": prompt, "stream": False},
        timeout=60,
    )
    response.raise_for_status()
    return response.json()["response"]


def format_duration(started_at, resolved_at):
    start = datetime.fromisoformat(started_at)
    end = datetime.fromisoformat(resolved_at)
    seconds = int((end - start).total_seconds())
    if seconds < 60:
        return f"{seconds} seconds"
    return f"{seconds // 60} minutes"


def build_prompt(event):
    name = event["monitor_name"]
    url = event["monitor_url"]

    instruction = (
        "You are a monitoring assistant. In 2-3 short sentences, explain to the "
        "site owner in plain language what likely happened, list probable causes "
        "as possibilities (not facts), and suggest one thing to check. Avoid jargon. "
        "Do not claim to know the exact cause. Reply with only the explanation."
    )

    if event["resolved"]:
        duration = format_duration(event["started_at"], event["resolved_at"])
        details = (
            f"Website '{name}' ({url}) went down and has now recovered. "
            f"It was unreachable for {duration}."
        )
    else:
        details = f"Website '{name}' ({url}) is currently down and not responding."

    return instruction + "\n\n" + details


def explain_incident(event):
    prompt = build_prompt(event)
    return explain(prompt)