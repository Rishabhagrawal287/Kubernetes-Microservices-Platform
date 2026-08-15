#!/bin/bash
echo "Switching to AI testing mode..."
kubectl scale deployment kube-prometheus-stack-grafana -n monitoring --replicas=0
kubectl scale statefulset prometheus-kube-prometheus-stack-prometheus -n monitoring --replicas=0
echo "Grafana + Prometheus scaled down. Ollama/log-analyzer have more room now."
