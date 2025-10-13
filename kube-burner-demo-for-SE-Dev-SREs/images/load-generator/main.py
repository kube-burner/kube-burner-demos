import asyncio
import json
import math
import os
import random
import string
import time
from typing import List

import httpx
from prometheus_client import Counter, Gauge, Histogram, start_http_server

TOTAL_REQUESTS = Counter(
    "lg_sent_requests_total",
    "Total number of requests sent by the load generator",
)
ERRORS_TOTAL = Counter(
    "lg_errors_total",
    "Total number of failed requests",
)
REQUEST_DURATION = Histogram(
    "lg_request_duration_seconds",
    "Histogram of request durations",
)
CURRENT_RATE = Gauge(
    "lg_current_rps",
    "Current target requests per second",
)


def env_float(name: str, default: float) -> float:
    raw = os.getenv(name)
    if raw is None:
        return default
    try:
        return float(raw)
    except ValueError:
        return default


def env_int(name: str, default: int) -> int:
    raw = os.getenv(name)
    if raw is None:
        return default
    try:
        return int(raw)
    except ValueError:
        return default


def env_actions(name: str, default: List[str]) -> List[str]:
    raw = os.getenv(name)
    if not raw:
        return default
    values = [item.strip() for item in raw.split(",") if item.strip()]
    return values or default


def env_seconds(name: str, default: float) -> float:
    raw = os.getenv(name)
    if raw is None:
        return default
    try:
        return float(raw)
    except ValueError:
        return default


def random_session_id(length: int = 12) -> str:
    alphabet = string.ascii_letters + string.digits
    return "".join(random.choice(alphabet) for _ in range(length))


async def send_interaction(client: httpx.AsyncClient, target: str, action: str, session_id: str) -> None:
    payload = {"action": action, "sessionId": session_id}
    start = time.perf_counter()
    try:
        response = await client.post(target, json=payload, headers={"x-session-id": session_id})
    except httpx.HTTPError:
        ERRORS_TOTAL.inc()
        return

    duration = time.perf_counter() - start
    REQUEST_DURATION.observe(duration)

    if response.status_code >= 300:
        ERRORS_TOTAL.inc()
    else:
        TOTAL_REQUESTS.inc()


async def main() -> None:
    target = os.getenv("TARGET_URL")
    if not target:
        raise SystemExit("TARGET_URL environment variable is required")

    actions = env_actions("ACTIONS", ["book_ticket", "cancel_ticket", "give_feedback"])
    base_rps = env_float("BASE_RPS", 2.0)
    ramp_factor = env_float("RAMP_FACTOR", 1.35)
    ramp_interval_seconds = env_seconds("RAMP_INTERVAL_SECONDS", 60.0)
    run_duration_seconds = env_seconds("RUN_DURATION_SECONDS", 300.0)
    metrics_port = env_int("METRICS_PORT", 2112)

    # Start Prometheus metrics server
    start_http_server(metrics_port)
    print(
        f"Launching load generator targeting {target} | base_rps={base_rps} "
        f"ramp_factor={ramp_factor} interval={ramp_interval_seconds}s duration={run_duration_seconds}s"
    )

    async with httpx.AsyncClient(timeout=httpx.Timeout(10.0, connect=5.0)) as client:
        start_time = time.perf_counter()
        end_time = start_time + run_duration_seconds

        while True:
            now = time.perf_counter()
            if now >= end_time:
                break

            elapsed = now - start_time
            exponent = elapsed / max(1.0, ramp_interval_seconds)
            current_rps = base_rps * math.pow(ramp_factor, exponent)
            CURRENT_RATE.set(current_rps)

            interval = 1.0
            requests_this_interval = max(1, int(round(current_rps * interval)))
            tasks = []
            for _ in range(requests_this_interval):
                action = random.choice(actions)
                session_id = random_session_id()
                tasks.append(asyncio.create_task(send_interaction(client, target, action, session_id)))

            if tasks:
                await asyncio.gather(*tasks)

            remaining = interval - (time.perf_counter() - now)
            if remaining > 0:
                await asyncio.sleep(remaining)

    print(
        json.dumps(
            {
                "target": target,
                "total_requests": TOTAL_REQUESTS._value.get(),
                "errors": ERRORS_TOTAL._value.get(),
                "runtime_seconds": run_duration_seconds,
            }
        )
    )


if __name__ == "__main__":
    asyncio.run(main())
