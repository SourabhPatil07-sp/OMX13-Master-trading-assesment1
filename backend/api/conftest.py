import pytest
import asyncio
from sqlalchemy.ext.asyncio import create_async_engine, AsyncSession, async_sessionmaker
from database import Base, settings
import models

@pytest.fixture(scope="session")
def event_loop():
    loop = asyncio.get_event_loop_policy().new_event_loop()
    yield loop
    loop.close()

@pytest.fixture(scope="session", autouse=True)
async def setup_database():
    engine = create_async_engine(settings.database_url)
    async with engine.begin() as conn:
        # For tests, we could recreate, but let's just ensure user 1 exists
        # as we are running against the dev DB in the container
        pass
    await engine.dispose()
