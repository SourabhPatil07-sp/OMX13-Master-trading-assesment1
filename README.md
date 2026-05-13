# OmniMarket

OmniMarket is an enterprise-grade prediction market platform featuring a dual matching architecture. It utilizes a Central Limit Order Book (CLOB) for exact peer-to-peer trades, with an automated fallback to an LMSR (Logarithmic Market Scoring Rule) Automated Market Maker (AMM) to ensure constant liquidity.

## 🏗 Architecture

The platform is built on a modern, microservice-style stack orchestrated entirely via Docker:

1. **Trading Engine (Go)**: A high-performance matching engine running on **Port 8080**. Handles real-time order matching, PostgreSQL row-level locking for atomic transactions, and WebSocket broadcasts (`/api/ws`) for live price updates.
2. **API & Admin Layer (FastAPI / Python)**: Running on **Port 8000**. Handles RESTful API operations, database seeding, and administrative panels (`/admin`).
3. **Frontend (React + Vite)**: Running on **Port 5173**. A modern SPA that connects to the FastAPI backend for static data and the Go engine for real-time WebSocket trading.
4. **Database (PostgreSQL)**: Running on **Port 5432**. The single source of truth for all markets, orders, and trades, ensuring strict consistency and concurrency control.

---

## 🚀 Getting Started

### Prerequisites
You only need two things installed on your system to run the entire stack:
- **Docker** & **Docker Compose**
- **Make** (Optional, but highly recommended for the Control Tower)

### 1. Build and Start the Stack

The easiest way to start the project is using the included `Makefile` control tower:

```bash
# Build the containers and start them in detached mode
make build

# If you don't have Make installed, you can run:
docker-compose up -d --build
```

### 2. Seed the Database

Once the database is running, you must populate it with initial categories, markets, and the AMM initial state.

```bash
make seed
```
*(This executes the `seed_db.py` script inside the FastAPI container).*

### 3. Access the Applications

- **Frontend Application**: [http://localhost:5173](http://localhost:5173)
- **FastAPI Documentation**: [http://localhost:8000/docs](http://localhost:8000/docs)
- **Go Engine (WebSockets)**: `ws://localhost:8080/api/ws`

---

## 🛠 Control Tower (Makefile)

The `Makefile` acts as a control tower for managing the platform. You can see all available commands by simply running `make` or `make help`:

| Command | Description |
|---------|-------------|
| `make up` | Starts all services in the background. |
| `make down` | Stops all services. |
| `make build` | Rebuilds containers (run this after adding new dependencies). |
| `make logs` | Tails the logs for all services combined. |
| `make seed` | Populates the database with initial MVP data. |
| `make psql` | Opens a direct `psql` shell into the PostgreSQL container. |
| `make test-engine` | Run the Go trading engine test suite. |
| `make test-engine-race` | Run the Go engine tests with the race detector. |
| `make test-api` | Run the FastAPI backend test suite via Docker. |
| `make test-frontend` | Run the React Vite test suite locally. |
| `make test-all` | Run all test suites sequentially. |
| `make benchmark-engine` | Run the Go trading engine benchmark suite. |
| `make clean` | Tears down the containers **and wipes the database volumes**. |

---

## 🧪 Testing the Trading Engine

The Go Trading Engine includes an extensive test suite to validate LMSR mathematics and high-volume concurrency database locks. 

Because the engine relies on the database, tests must run within the Docker network. The Makefile handles spawning temporary containers to run these natively:

```bash
# Run the standard test suite (extremely fast)
make test-engine

# Run the aggressive concurrency stress test with Go's Data Race Detector (slower)
make test-engine-race
```

The stress test spins up **5,000 concurrent goroutines** simulating high-frequency traders hitting the database simultaneously to verify strict ACID compliance.

### API & Frontend Tests
We also have dedicated tests for the other layers:
```bash
# Test the FastAPI REST endpoints inside the running Docker container
make test-api

# Test the React UI locally
make test-frontend

# Run all test suites across the stack sequentially
make test-all
```

---

## 📊 Observability

The platform includes built-in observability features designed to provide real-time insights without requiring external tools:

- **FastAPI Middleware**: All incoming HTTP requests are logged as JSON objects including `method`, `endpoint`, `status_code`, and `latency`. Requests taking longer than 300ms trigger an automatic `WARNING` log.
- **Go Engine Trade Logging**: The matching engine logs every single trade action as a structured JSON object. It records:
  - `event`: "trade"
  - Execution Engine (`CLOB` vs `LMSR`)
  - Order `quantity` and `remaining_quantity`
  - Total `db_time` to track PostgreSQL performance
  - For AMM fallbacks, it records `price_before` and `price_after` to track slippage.
- **Error Tracking**: Global exception handlers automatically catch and format unhandled errors into JSON logs to ensure easy aggregation.

---

## 💡 Core Trading Concepts

- **Dual Architecture**: Every order first attempts to match against resting counter-orders in the CLOB. Any unfilled remainder (for `MARKET` orders) is mathematically priced and fulfilled by the LMSR AMM pool.
- **Limit vs Market**: 
  - `LIMIT` orders provide liquidity (Makers). If they do not match, they stay `OPEN` in the database.
  - `MARKET` orders consume liquidity (Takers). They cross the CLOB and route any remainder to the AMM pool, ensuring instant execution.
- **LMSR**: The AMM uses the Logarithmic Market Scoring Rule, where the price of a share scales logarithmically based on the ratio of existing `YES` and `NO` shares in the pool, controlled by the liquidity constant `B`.
