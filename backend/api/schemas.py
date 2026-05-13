from pydantic import BaseModel, ConfigDict
from datetime import datetime
from typing import Optional, List
from decimal import Decimal

class UserBase(BaseModel):
    username: str

class UserCreate(UserBase):
    pass

class UserResponse(UserBase):
    id: int
    balance: Decimal
    model_config = ConfigDict(from_attributes=True)

class CategoryResponse(BaseModel):
    id: int
    name: str
    description: Optional[str] = None
    model_config = ConfigDict(from_attributes=True)

class MarketBase(BaseModel):
    question: str
    expiry: datetime
    category_id: int

class MarketCreate(MarketBase):
    b_parameter: Optional[Decimal] = Decimal("100.00")

class MarketResponse(MarketBase):
    id: int
    is_resolved: bool
    resolved_outcome: Optional[str] = None
    b_parameter: Decimal
    created_at: datetime
    category: Optional[CategoryResponse] = None
    model_config = ConfigDict(from_attributes=True)

class OrderBase(BaseModel):
    market_id: int
    outcome: str
    order_type: str = "LIMIT"
    price: Decimal
    shares: Decimal

class OrderCreate(OrderBase):
    pass

class OrderResponse(OrderBase):
    id: int
    user_id: int
    filled_shares: Decimal
    status: str
    created_at: datetime
    model_config = ConfigDict(from_attributes=True)

class TradeResponse(BaseModel):
    id: int
    market_id: int
    price: Decimal
    shares: Decimal
    executed_at: datetime
    model_config = ConfigDict(from_attributes=True)

class OrderbookLevel(BaseModel):
    price: float
    shares: float

class OrderbookResponse(BaseModel):
    yes_orders: List[OrderbookLevel]
    no_orders: List[OrderbookLevel]

class AmmPriceResponse(BaseModel):
    price_yes: float
    price_no: float
