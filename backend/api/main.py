from fastapi import FastAPI, Depends, HTTPException, Request
from fastapi.responses import JSONResponse
from sqlalchemy.ext.asyncio import AsyncSession
from sqlalchemy.future import select
from sqlalchemy.orm import selectinload
from typing import List
from sqladmin import Admin, ModelView
import time
import logging
import json

import models, schemas
from database import engine, get_db, Base

from fastapi.middleware.cors import CORSMiddleware

import math

# Configure JSON Logging
class JSONFormatter(logging.Formatter):
    def format(self, record):
        log_obj = {"level": record.levelname, "msg": record.getMessage()}
        if hasattr(record, "extra_data"):
            log_obj.update(record.extra_data)
        return json.dumps(log_obj)

logger = logging.getLogger("fastapi_observability")
logger.setLevel(logging.INFO)
ch = logging.StreamHandler()
ch.setFormatter(JSONFormatter())
logger.addHandler(ch)

app = FastAPI(title="OmniMarket X API")

@app.middleware("http")
async def log_request_middleware(request: Request, call_next):
    start_time = time.time()
    try:
        response = await call_next(request)
    except Exception as exc:
        latency_ms = (time.time() - start_time) * 1000
        log_data = {
            "event": "error",
            "endpoint": request.url.path,
            "error_message": str(exc),
            "latency": round(latency_ms, 2)
        }
        logger.error("Unhandled exception", extra={"extra_data": log_data})
        raise exc
    
    latency_ms = (time.time() - start_time) * 1000
    log_data = {
        "event": "request",
        "method": request.method,
        "endpoint": request.url.path,
        "status_code": response.status_code,
        "latency": round(latency_ms, 2)
    }
    
    if latency_ms > 300:
        logger.warning("High latency request", extra={"extra_data": log_data})
    else:
        logger.info("Request processed", extra={"extra_data": log_data})
        
    return response

@app.exception_handler(Exception)
async def global_exception_handler(request: Request, exc: Exception):
    # Exception is already logged in middleware with latency, but in case it's thrown before middleware (unlikely)
    return JSONResponse(status_code=500, content={"detail": "Internal Server Error"})

app.add_middleware(
    CORSMiddleware,
    allow_origins=["*"],
    allow_credentials=True,
    allow_methods=["*"],
    allow_headers=["*"],
)

# SQLAdmin Configuration
admin = Admin(app, engine)

class CategoryAdmin(ModelView, model=models.Category):
    column_list = [models.Category.id, models.Category.name]

class MarketAdmin(ModelView, model=models.Market):
    column_list = [models.Market.id, models.Market.question, models.Market.category_id, models.Market.is_resolved]

class UserAdmin(ModelView, model=models.User):
    column_list = [models.User.id, models.User.username, models.User.balance]

class OrderAdmin(ModelView, model=models.Order):
    column_list = [models.Order.id, models.Order.user_id, models.Order.market_id, models.Order.outcome, models.Order.status]

class TradeAdmin(ModelView, model=models.Trade):
    column_list = [models.Trade.id, models.Trade.market_id, models.Trade.price, models.Trade.shares]

admin.add_view(CategoryAdmin)
admin.add_view(MarketAdmin)
admin.add_view(UserAdmin)
admin.add_view(OrderAdmin)
admin.add_view(TradeAdmin)

@app.on_event("startup")
async def startup():
    async with engine.begin() as conn:
        await conn.run_sync(models.Base.metadata.create_all)

@app.post("/auth/login", response_model=schemas.UserResponse)
async def login(user: schemas.UserCreate, db: AsyncSession = Depends(get_db)):
    result = await db.execute(select(models.User).filter(models.User.username == user.username))
    db_user = result.scalars().first()
    if not db_user:
        # For MVP simple login, create if doesn't exist
        db_user = models.User(username=user.username)
        db.add(db_user)
        await db.commit()
        await db.refresh(db_user)
    return db_user

@app.post("/admin/markets", response_model=schemas.MarketResponse)
async def create_market(market: schemas.MarketCreate, db: AsyncSession = Depends(get_db)):
    db_market = models.Market(**market.model_dump())
    db.add(db_market)
    await db.flush() # To get market ID
    
    # Initialize AMM State
    amm_state = models.AmmState(market_id=db_market.id, q_yes=0.00, q_no=0.00)
    db.add(amm_state)
    await db.commit()
    await db.refresh(db_market)
    
    # Reload with category eager loaded to return full schema
    result = await db.execute(select(models.Market).options(selectinload(models.Market.category)).filter(models.Market.id == db_market.id))
    return result.scalars().first()

@app.get("/markets", response_model=List[schemas.MarketResponse])
async def list_markets(category_id: int = None, db: AsyncSession = Depends(get_db)):
    query = select(models.Market).options(selectinload(models.Market.category))
    if category_id:
        query = query.filter(models.Market.category_id == category_id)
    result = await db.execute(query)
    markets = result.scalars().all()
    return markets

@app.get("/markets/{market_id}", response_model=schemas.MarketResponse)
async def get_market(market_id: int, db: AsyncSession = Depends(get_db)):
    result = await db.execute(select(models.Market).options(selectinload(models.Market.category)).filter(models.Market.id == market_id))
    market = result.scalars().first()
    if not market:
        raise HTTPException(status_code=404, detail="Market not found")
    return market

@app.get("/markets/{market_id}/trades", response_model=List[schemas.TradeResponse])
async def list_market_trades(market_id: int, db: AsyncSession = Depends(get_db)):
    result = await db.execute(
        select(models.Trade)
        .filter(models.Trade.market_id == market_id)
        .order_by(models.Trade.executed_at.asc())
    )
    return result.scalars().all()

@app.get("/markets/{market_id}/amm", response_model=schemas.AmmPriceResponse)
async def get_amm_price(market_id: int, db: AsyncSession = Depends(get_db)):
    result = await db.execute(select(models.AmmState).filter(models.AmmState.market_id == market_id))
    amm_state = result.scalars().first()
    result = await db.execute(select(models.Market).filter(models.Market.id == market_id))
    market = result.scalars().first()
    if not amm_state or not market:
        raise HTTPException(status_code=404, detail="AMM State not found")
    b = float(market.b_parameter)
    q_yes = float(amm_state.q_yes)
    q_no = float(amm_state.q_no)
    exp_yes = math.exp(q_yes / b)
    exp_no = math.exp(q_no / b)
    price_yes = exp_yes / (exp_yes + exp_no)
    price_no = exp_no / (exp_yes + exp_no)
    return {"price_yes": price_yes, "price_no": price_no}

@app.get("/markets/{market_id}/orderbook", response_model=schemas.OrderbookResponse)
async def get_orderbook(market_id: int, db: AsyncSession = Depends(get_db)):
    result = await db.execute(
        select(models.Order)
        .filter(models.Order.market_id == market_id, models.Order.status == "OPEN")
        .order_by(models.Order.price.desc())
    )
    orders = result.scalars().all()
    yes_orders = []
    no_orders = []
    for o in orders:
        level = {"price": float(o.price) / 100, "shares": float(o.shares - o.filled_shares)}
        if o.outcome == "YES":
            yes_orders.append(level)
        else:
            no_orders.append(level)
    return {"yes_orders": yes_orders[:10], "no_orders": no_orders[:10]}

@app.get("/users/{user_id}/profile", response_model=schemas.UserResponse)
async def get_user_profile(user_id: int, db: AsyncSession = Depends(get_db)):
    result = await db.execute(select(models.User).filter(models.User.id == user_id))
    user = result.scalars().first()
    if not user:
        raise HTTPException(status_code=404, detail="User not found")
    return user
