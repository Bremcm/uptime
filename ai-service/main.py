import json
from kafka import KafkaConsumer
from llm import explain_incident


def main():
    consumer = KafkaConsumer(
        "incidents",
        bootstrap_servers="localhost:9092",
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
        except Exception as e:
            print(f"failed to get explanation: {e}")


if __name__ == "__main__":
    main()