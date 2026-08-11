#!/usr/bin/env bash
# Automates the exact test sequence we've been running by hand throughout
# this project: register a user, seed a product, create an order, and
# confirm the RabbitMQ event actually decremented stock. If this script
# fails, the event-driven flow between order-service and product-service is
# broken — this is precisely the kind of regression (e.g. the "consumer
# connects once and gives up" bug from Phase 2a) that manual testing can
# miss but an automated check like this catches every time.
set -euo pipefail

NAMESPACE="microservices"
PRODUCT_ID="ci-prod-1"

echo "Starting port-forwards..."
kubectl port-forward -n "$NAMESPACE" svc/user-service 3001:3000 >/tmp/pf-user.log 2>&1 &
PF_USER=$!
kubectl port-forward -n "$NAMESPACE" svc/order-service 3002:8000 >/tmp/pf-order.log 2>&1 &
PF_ORDER=$!
kubectl port-forward -n "$NAMESPACE" svc/product-service 3003:8080 >/tmp/pf-product.log 2>&1 &
PF_PRODUCT=$!

cleanup() {
  kill "$PF_USER" "$PF_ORDER" "$PF_PRODUCT" 2>/dev/null || true
}
trap cleanup EXIT

echo "Waiting for port-forwards to establish..."
sleep 5

echo "1. Registering a test user..."
curl -sf -X POST http://localhost:3001/api/users/register \
  -H "Content-Type: application/json" \
  -d '{"email":"ci-test@example.com","password":"password123"}'
echo ""

echo "2. Seeding a test product with stock=100..."
curl -sf -X POST http://localhost:3003/api/products \
  -H "Content-Type: application/json" \
  -d "{\"id\":\"${PRODUCT_ID}\",\"name\":\"CI Test Widget\",\"price\":9.99,\"stock\":100}"
echo ""

echo "3. Creating an order for quantity=5..."
curl -sf -X POST http://localhost:3002/api/orders \
  -H "Content-Type: application/json" \
  -d "{\"user_id\":\"ci-user\",\"product_id\":\"${PRODUCT_ID}\",\"quantity\":5}"
echo ""

echo "4. Waiting for the RabbitMQ event to be consumed..."
sleep 5

echo "5. Checking stock actually decremented..."
RESPONSE=$(curl -sf "http://localhost:3003/api/products/${PRODUCT_ID}")
echo "Product response: ${RESPONSE}"

STOCK=$(echo "${RESPONSE}" | grep -o '"stock":[0-9]*' | cut -d: -f2)

if [ "${STOCK}" != "95" ]; then
  echo "FAILED: expected stock=95, got stock=${STOCK} — the event-driven flow is broken"
  exit 1
fi

echo "PASSED: full event-driven flow verified end to end (order -> RabbitMQ -> stock decrement)"
