import json
import logging
import os

import pika

logger = logging.getLogger("order-service.rabbitmq")

RABBITMQ_URL = os.getenv("RABBITMQ_URL", "amqp://guest:guest@localhost:5672/")
EXCHANGE_NAME = "order_events"


def publish_order_created(order_id: int, product_id: str, quantity: int) -> None:
    """Publish an 'order.created' event.

    Failures are logged, not raised — a broker hiccup shouldn't fail the HTTP
    request that already committed the order to Postgres. Product-service
    inventory will simply lag until the broker is reachable again.
    """
    try:
        connection = pika.BlockingConnection(pika.URLParameters(RABBITMQ_URL))
        channel = connection.channel()
        channel.exchange_declare(exchange=EXCHANGE_NAME, exchange_type="fanout", durable=True)

        payload = json.dumps(
            {
                "event": "order.created",
                "order_id": order_id,
                "product_id": product_id,
                "quantity": quantity,
            }
        )
        channel.basic_publish(exchange=EXCHANGE_NAME, routing_key="", body=payload)
        connection.close()
        logger.info("published order.created for order_id=%s", order_id)
    except Exception as exc:  # noqa: BLE001 — broker issues shouldn't crash the request
        logger.error("failed to publish order.created: %s", exc)
