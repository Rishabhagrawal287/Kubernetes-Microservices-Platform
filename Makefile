.PHONY: up down build logs ps clean kind-up kind-down build-images kind-load k8s-infra helm-install helm-uninstall k8s-status

## ---- Docker Compose (local dev) ----

up:
	docker compose up -d --build

down:
	docker compose down

build:
	docker compose build

logs:
	docker compose logs -f

ps:
	docker compose ps

clean:
	docker compose down -v --remove-orphans

## ---- Kind (local Kubernetes) ----

kind-up:
	kind create cluster --config infra/kind-config.yaml

kind-down:
	kind delete cluster --name microservices-platform

## ---- Phase 2b: Helm + Kubernetes ----

build-images:
	docker build -t user-service:local ./services/user-service
	docker build -t order-service:local ./services/order-service
	docker build -t product-service:local ./services/product-service

kind-load: build-images
	kind load docker-image user-service:local --name microservices-platform
	kind load docker-image order-service:local --name microservices-platform
	kind load docker-image product-service:local --name microservices-platform

k8s-infra:
	kubectl apply -f k8s/infra/namespace.yaml
	kubectl apply -f k8s/infra/secrets.yaml
	kubectl apply -f k8s/infra/mongo.yaml
	kubectl apply -f k8s/infra/postgres.yaml
	kubectl apply -f k8s/infra/redis.yaml
	kubectl apply -f k8s/infra/rabbitmq.yaml

helm-install:
	helm install user-service ./helm/user-service -n microservices -f ./helm/user-service/values-dev.yaml
	helm install order-service ./helm/order-service -n microservices -f ./helm/order-service/values-dev.yaml
	helm install product-service ./helm/product-service -n microservices -f ./helm/product-service/values-dev.yaml

helm-upgrade:
	helm upgrade user-service ./helm/user-service -n microservices -f ./helm/user-service/values-dev.yaml
	helm upgrade order-service ./helm/order-service -n microservices -f ./helm/order-service/values-dev.yaml
	helm upgrade product-service ./helm/product-service -n microservices -f ./helm/product-service/values-dev.yaml

helm-uninstall:
	-helm uninstall user-service -n microservices
	-helm uninstall order-service -n microservices
	-helm uninstall product-service -n microservices

k8s-status:
	kubectl get all -n microservices

## ---- Phase 4a: Observability (Prometheus + Grafana) ----

observability-repo:
	helm repo add prometheus-community https://prometheus-community.github.io/helm-charts
	helm repo update

observability-install: observability-repo
	helm install kube-prometheus-stack prometheus-community/kube-prometheus-stack \
		-n monitoring --create-namespace \
		-f k8s/observability/kube-prometheus-stack-values.yaml
	kubectl apply -f k8s/observability/alert-rules.yaml
	kubectl apply -f k8s/observability/grafana-dashboards/microservices-platform-dashboard.yaml

observability-uninstall:
	-helm uninstall kube-prometheus-stack -n monitoring

observability-status:
	kubectl get pods,svc -n monitoring

grafana-forward:
	kubectl port-forward -n monitoring svc/kube-prometheus-stack-grafana 3000:80
