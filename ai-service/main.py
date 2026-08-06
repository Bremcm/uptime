import os
import json
from kafka import KafkaConsumer
from dotenv import load_dotenv
from llm import explain_incident
from telegram import send_message


load_dotenv(dotenv_path="../.env")
TELEGRAM_TOKEN = os.environ["TELEGRAM_TOKEN"]


def main():
    brokers = os.environ.get("KAFKA_BROKERS", "localhost:9092")
    consumer = KafkaConsumer(
        "incidents",
        bootstrap_servers=brokers,
        group_id="ai-explainer",
        auto_offset_reset="earliest",
        value_deserializer=lambda m: json.loads(m.decode("utf-8")),
    )

    print("ai-service started, waiting for incidents...")

    for message in consumer:
        event = message.value
        state = "resolved" if event["resolved"] else "down"
        print(f"\n--- incident {event['incident_id']} ({state}) ---")

        try:
            explanation = explain_incident(event)
            print(explanation)

            chat_id = event["chat_id"]
            if chat_id:
                send_message(TELEGRAM_TOKEN, chat_id, f"🤖 {explanation}")
        except Exception as e:
            print(f"failed to process incident: {e}")


if __name__ == "__main__":
    main()