from sqlalchemy import Column, Integer, String, Boolean, Numeric, ForeignKey, DateTime
from sqlalchemy.orm import relationship
from sqlalchemy.sql import func
from database import Base

class User(Base):
    __tablename__ = "users"
    id = Column(Integer, primary_key=True, index=True)
    username = Column(String, unique=True, index=True, nullable=False)
    balance = Column(Numeric(10, 2), default=1000.00)
    
class Category(Base):
    __tablename__ = "categories"
    id = Column(Integer, primary_key=True, index=True)
    name = Column(String, unique=True, index=True, nullable=False)
    description = Column(String, nullable=True)

class Market(Base):
    __tablename__ = "markets"
    id = Column(Integer, primary_key=True, index=True)
    question = Column(String, nullable=False)
    expiry = Column(DateTime(timezone=True), nullable=False)
    category_id = Column(Integer, ForeignKey("categories.id"), nullable=False)
    is_resolved = Column(Boolean, default=False)
    resolved_outcome = Column(String, nullable=True)
    b_parameter = Column(Numeric(10, 2), default=100.00)
    created_at = Column(DateTime(timezone=True), server_default=func.now())

    category = relationship("Category")
    
class AmmState(Base):
    __tablename__ = "amm_state"
    id = Column(Integer, primary_key=True, index=True)
    market_id = Column(Integer, ForeignKey("markets.id"), unique=True)
    q_yes = Column(Numeric(10, 2), default=0.00)
    q_no = Column(Numeric(10, 2), default=0.00)

class Order(Base):
    __tablename__ = "orders"
    id = Column(Integer, primary_key=True, index=True)
    user_id = Column(Integer, ForeignKey("users.id"))
    market_id = Column(Integer, ForeignKey("markets.id"))
    outcome = Column(String, nullable=False)
    order_type = Column(String, default="LIMIT")
    price = Column(Numeric(10, 2), nullable=False)
    shares = Column(Numeric(10, 2), nullable=False)
    filled_shares = Column(Numeric(10, 2), default=0.00)
    status = Column(String, default="OPEN")
    created_at = Column(DateTime(timezone=True), server_default=func.now())

class Trade(Base):
    __tablename__ = "trades"
    id = Column(Integer, primary_key=True, index=True)
    market_id = Column(Integer, ForeignKey("markets.id"))
    maker_order_id = Column(Integer, ForeignKey("orders.id"), nullable=True)
    taker_order_id = Column(Integer, ForeignKey("orders.id"))
    price = Column(Numeric(10, 2), nullable=False)
    shares = Column(Numeric(10, 2), nullable=False)
    executed_at = Column(DateTime(timezone=True), server_default=func.now())
