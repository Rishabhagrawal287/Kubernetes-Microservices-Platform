import os
import time
import requests
from fastapi import FastAPI, HTTPException

app = FastAPI(title="log-analyzer")

LOKI_URL = os.getenv("LOKI_URL", "http://loki-gateway.monitoring.svc.cluster.local:80")
OLLAMA_URL = os.getenv("OLLAMA_URL", "http://ollama.ai.svc.cluster.local:11434")
MODEL = os.getenv("OLLAMA_MODEL", "tinyllama")

PROMPT_TEMPLATE = """Log: {log_line}
Category: [Database/Network/Auth/Application/Other]
Severity: [Low/Medium/High/Critical]
One-line summary:"""


@app.get("/health")
def health():
    return {"status": "ok", "service": "log-analyzer"}


@app.get("/ready")
def ready():
    return {"status": "ready", "service": "log-analyzer"}


def fetch_recent_logs(namespace: str = "microservices", limit: int = 20):
    params = {
        "query": f'{{namespace="{namespace}"}} |~ "(?i)error|exception| failed| failure|connection timeout| [45][0-9][0-9] "',
        "limit": limit,
    }
    resp = requests.get(f"{LOKI_URL}/loki/api/v1/query_range", params=params, timeout=10)
    resp.raise_for_status()
    data = resp.json()
    lines = []
    for stream in data.get("data", {}).get("result", []):
        for value in stream.get("values", []):
            lines.append(value[1])
    return lines


def analyze_log(log_line: str):
    prompt = PROMPT_TEMPLATE.format(log_line=log_line.strip()[:200])
    payload = {
        "model": MODEL,
        "prompt": prompt,
        "stream": False,
        "options": {"temperature": 0.2},
    }
    resp = requests.post(f"{OLLAMA_URL}/api/generate", json=payload, timeout=60)
    resp.raise_for_status()
    return resp.json().get("response", "").strip()


@app.get("/analyze")
def analyze(namespace: str = "microservices", limit: int = 5):
    try:
        logs = fetch_recent_logs(namespace=namespace, limit=limit)
    except Exception as e:
        raise HTTPException(status_code=502, detail=f"Loki query failed: {e}")

    if not logs:
        return {"analyzed": 0, "results": []}

    results = []
    for line in logs[:limit]:
        try:
            analysis = analyze_log(line)
        except Exception as e:
            analysis = f"analysis failed: {e}"
        results.append({"log": line.strip(), "analysis": analysis})

    return {"analyzed": len(results), "results": results}
