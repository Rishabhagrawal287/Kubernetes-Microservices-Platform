import time

from fastapi import Request
from prometheus_client import CONTENT_TYPE_LATEST, Counter, Histogram, generate_latest

# ---------- Golden signals (traffic, errors, latency) ----------
http_requests_total = Counter(
    "http_requests_total", "Total HTTP requests", ["method", "path", "status_code"]
)
http_request_duration_seconds = Histogram(
    "http_request_duration_seconds", "HTTP request duration in seconds", ["method", "path"]
)

# ---------- Business metric ----------
orders_created_total = Counter(
    "orders_created_total", "Total number of orders created"
)


async def metrics_middleware(request: Request, call_next):
    start = time.perf_counter()
    response = await call_next(request)
    duration = time.perf_counter() - start
    path = request.url.path
    http_requests_total.labels(request.method, path, response.status_code).inc()
    http_request_duration_seconds.labels(request.method, path).observe(duration)
    return response


def metrics_response():
    return generate_latest(), CONTENT_TYPE_LATEST
