import asyncio
import json
import os
import random
from datetime import datetime, timedelta
from sqlalchemy.future import select

import models
from database import engine, AsyncSessionLocal

async def seed():
    print("Loading seed data...")
    with open("seed.json", "r") as f:
        data = json.load(f)

    async with AsyncSessionLocal() as session:
        # Insert Default User
        print("Inserting Default User...")
        result = await session.execute(select(models.User).filter_by(id=1))
        if not result.scalars().first():
            user = models.User(id=1, username="trader1", balance=10000.00)
            session.add(user)
            await session.commit()
            print("Default user 'trader1' created.")

        # Insert Categories
        print("Inserting Categories...")
        category_id_map = {}
        for cat_data in data.get("categories", []):
            original_id = cat_data.pop("id", None)
            cat_name = cat_data["name"]
            result = await session.execute(select(models.Category).filter_by(name=cat_name))
            existing_category = result.scalars().first()
            if not existing_category:
                category = models.Category(**cat_data)
                session.add(category)
                await session.flush()
                category_id_map[original_id] = category.id
                print(f"Created category '{cat_name}': {original_id} -> {category.id}")
            else:
                category_id_map[original_id] = existing_category.id
                print(f"Found existing category '{cat_name}': {original_id} -> {existing_category.id}")
        
        await session.commit()
        
        # Insert Markets and AmmState
        print("Inserting Markets...")
        for market_data in data.get("markets", []):
            market_id = market_data.pop("id", None)
            # Map the category_id from JSON to the actual DB ID
            json_cat_id = market_data.get("category_id")
            if json_cat_id in category_id_map:
                market_data["category_id"] = category_id_map[json_cat_id]
                # print(f"Mapped market category {json_cat_id} -> {market_data['category_id']}")
            else:
                print(f"Warning: Category ID {json_cat_id} not found in map!")
            
            result = await session.execute(select(models.Market).filter_by(question=market_data["question"]))
            if not result.scalars().first():
                # Parse datetime strings
                market_data["expiry"] = datetime.fromisoformat(market_data["expiry"].replace("Z", "+00:00"))
                market_data["created_at"] = datetime.fromisoformat(market_data["created_at"].replace("Z", "+00:00"))
                
                market = models.Market(**market_data)
                session.add(market)
                await session.flush()
                
                # Create corresponding AmmState with some initial random liquidity
                q_yes_init = random.uniform(0, 200)
                q_no_init = random.uniform(0, 200)
                amm = models.AmmState(market_id=market.id, q_yes=q_yes_init, q_no=q_no_init)
                session.add(amm)
                
        await session.commit()

        # Insert Mock Trades for the first few markets to show a chart
        print("Inserting Mock Trades...")
        result = await session.execute(select(models.Market).limit(5))
        markets = result.scalars().all()
        for market in markets:
            trade_check = await session.execute(select(models.Trade).filter_by(market_id=market.id))
            if not trade_check.scalars().first():
                base_price = 50.0
                for i in range(20):
                    base_price += random.uniform(-5, 5)
                    base_price = max(10, min(90, base_price))
                    trade = models.Trade(
                        market_id=market.id,
                        price=base_price,
                        shares=random.randint(10, 100),
                        executed_at=datetime.now() - timedelta(hours=20-i)
                    )
                    session.add(trade)
        
        await session.commit()
        print("Database seeded successfully!")

if __name__ == "__main__":
    asyncio.run(seed())
