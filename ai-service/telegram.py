import requests


TELEGRAM_API = "https://api.telegram.org/bot{token}/sendMessage"


def send_message(token, chat_id, text):
    url = TELEGRAM_API.format(token=token)
    response = requests.post(
        url,
        json={"chat_id": chat_id, "text": text},
        timeout=10,
    )
    response.raise_for_status()