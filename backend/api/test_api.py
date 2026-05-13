import pytest
import pytest_asyncio
from httpx import ASGITransport, AsyncClient
from main import app

@pytest_asyncio.fixture
async def client():
    async with AsyncClient(transport=ASGITransport(app=app), base_url="http://test") as ac:
        yield ac

@pytest.mark.asyncio
async def test_read_markets(client):
    response = await client.get("/markets")
    assert response.status_code == 200
    assert isinstance(response.json(), list)

@pytest.mark.asyncio
async def test_get_nonexistent_market(client):
    response = await client.get("/markets/999999")
    assert response.status_code == 404

@pytest.mark.asyncio
async def test_get_amm_price_valid_market(client):
    markets_res = await client.get("/markets")
    markets = markets_res.json()
    if len(markets) > 0:
        market_id = markets[0]["id"]
        response = await client.get(f"/markets/{market_id}/amm")
        assert response.status_code == 200
        data = response.json()
        assert "price_yes" in data
        assert "price_no" in data

@pytest.mark.asyncio
async def test_user_profile_exists(client):
    # This verifies User 1 exists (from our seeder fix)
    response = await client.get("/users/1/profile")
    assert response.status_code == 200
    data = response.json()
    assert data["id"] == 1
    assert float(data["balance"]) > 0

@pytest.mark.asyncio
async def test_orderbook_fetching(client):
    markets_res = await client.get("/markets")
    markets = markets_res.json()
    if len(markets) > 0:
        market_id = markets[0]["id"]
        response = await client.get(f"/markets/{market_id}/orderbook")
        assert response.status_code == 200
        data = response.json()
        assert "yes_orders" in data
        assert "no_orders" in data
