from datetime import datetime

from pydantic import BaseModel, Field


class OrderCreate(BaseModel):
    user_id: str = Field(..., examples=["user-123"])
    product_id: str = Field(..., examples=["prod-1"])
    quantity: int = Field(..., gt=0)


class OrderOut(BaseModel):
    id: int
    user_id: str
    product_id: str
    quantity: int
    status: str
    created_at: datetime

    class Config:
        from_attributes = True
