#!/bin/bash
echo "Switching to observability mode..."
kubectl scale deployment ollama -n ai --replicas=0
kubectl scale deployment kube-prometheus-stack-grafana -n monitoring --replicas=1
kubectl scale statefulset prometheus-kube-prometheus-stack-prometheus -n monitoring --replicas=1
echo "Ollama scaled down. Grafana + Prometheus restored."
