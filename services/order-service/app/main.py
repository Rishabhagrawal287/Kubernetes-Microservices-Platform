import logging

from fastapi import Depends, FastAPI, HTTPException, Response
from sqlalchemy import text
from sqlalchemy.orm import Session

from . import models, schemas
from .database import Base, engine, get_db
from .metrics import metrics_middleware, metrics_response, orders_created_total
from .rabbitmq import publish_order_created

# Without this, module-level loggers (like rabbitmq.py's) default to only
# showing WARNING+ — successful "published order.created" confirmations
# would be silently invisible in `docker compose logs`.
logging.basicConfig(level=logging.INFO)

Base.metadata.create_all(bind=engine)

app = FastAPI(title="order-service")
app.middleware("http")(metrics_middleware)


@app.get("/health")
def health():
    """Liveness: is the process up at all."""
    return {"status": "ok", "service": "order-service"}


@app.get("/ready")
def ready(db: Session = Depends(get_db)):
    """Readiness: only ready once Postgres actually responds."""
    try:
        db.execute(text("SELECT 1"))
        return {"status": "ready", "service": "order-service"}
    except Exception as exc:
        raise HTTPException(status_code=503, detail="database not reachable") from exc


@app.get("/metrics")
def metrics():
    """Scraped by Prometheus via the ServiceMonitor in helm/order-service."""
    body, content_type = metrics_response()
    return Response(content=body, media_type=content_type)


@app.post("/api/orders", response_model=schemas.OrderOut, status_code=201)
def create_order(order: schemas.OrderCreate, db: Session = Depends(get_db)):
    db_order = models.Order(
        user_id=order.user_id, product_id=order.product_id, quantity=order.quantity
    )
    db.add(db_order)
    db.commit()
    db.refresh(db_order)
    orders_created_total.inc()

    # Fire-and-forget: product-service consumes this to decrement stock
    publish_order_created(db_order.id, db_order.product_id, db_order.quantity)

    return db_order


@app.get("/api/orders/{order_id}", response_model=schemas.OrderOut)
def get_order(order_id: int, db: Session = Depends(get_db)):
    db_order = db.get(models.Order, order_id)
    if not db_order:
        raise HTTPException(status_code=404, detail="order not found")
    return db_order


@app.get("/api/orders", response_model=list[schemas.OrderOut])
def list_orders(db: Session = Depends(get_db)):
    return db.query(models.Order).order_by(models.Order.id.desc()).limit(100).all()
