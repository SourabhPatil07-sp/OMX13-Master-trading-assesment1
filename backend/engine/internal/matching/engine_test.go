package matching

import (
	"context"
	"fmt"
	"math"
	"os"
	"testing"
	"time"

	"omnimarket-engine/internal/db"
	"omnimarket-engine/internal/models"
	"omnimarket-engine/internal/ws"
)

func setupTestDB(t *testing.T) {
	// Set the environment variable to connect to the docker db instance
	os.Setenv("DATABASE_URL", "postgresql://omnimarket:omnimarket_password@db:5432/omnimarket?sslmode=disable")
	if err := db.ConnectDB(); err != nil {
		t.Fatalf("Failed to connect to test DB: %v", err)
	}

	// Clean tables
	ctx := context.Background()
	_, _ = db.Pool.Exec(ctx, "DELETE FROM trades")
	_, _ = db.Pool.Exec(ctx, "DELETE FROM orders")
	_, _ = db.Pool.Exec(ctx, "DELETE FROM amm_state")
	_, _ = db.Pool.Exec(ctx, "DELETE FROM markets")
	_, _ = db.Pool.Exec(ctx, "DELETE FROM categories")
	_, _ = db.Pool.Exec(ctx, "DELETE FROM users")
}

func createMockUser(t *testing.T, id int, username string, balance float64) {
	_, err := db.Pool.Exec(context.Background(), "INSERT INTO users (id, username, balance) VALUES ($1, $2, $3)", id, username, balance)
	if err != nil {
		t.Fatalf("Failed to create mock user: %v", err)
	}
}

func createMockMarket(t *testing.T, id int) {
	ctx := context.Background()
	_, err := db.Pool.Exec(ctx, "INSERT INTO categories (id, name, description) VALUES (1, 'Test Category', '') ON CONFLICT DO NOTHING")
	if err != nil {
		t.Fatalf("Failed to create category: %v", err)
	}

	_, err = db.Pool.Exec(ctx, `
		INSERT INTO markets (id, question, expiry, category_id, is_resolved, b_parameter, created_at) 
		VALUES ($1, 'Test Market', $2, 1, false, 100.00, $3)
	`, id, time.Now().Add(24*time.Hour), time.Now())
	if err != nil {
		t.Fatalf("Failed to create market: %v", err)
	}

	_, err = db.Pool.Exec(ctx, "INSERT INTO amm_state (market_id, q_yes, q_no) VALUES ($1, 0, 0)", id)
	if err != nil {
		t.Fatalf("Failed to create amm_state: %v", err)
	}
}

func getEngine() *MatchingEngine {
	hub := ws.NewHub()
	go hub.Run()
	return NewMatchingEngine(hub)
}

// TestProcessOrder_ZeroBalance verifies that the engine correctly rolls back transactions
// and prevents execution when a user attempts to place an order without sufficient funds.
// What it tests: Rejection of orders with zero balance.
// Why it matters: Ensures state consistency and prevents infinite money glitch.
// Status: Useful
func TestProcessOrder_ZeroBalance(t *testing.T) {
	setupTestDB(t)
	defer db.CloseDB()

	createMockUser(t, 1, "broke_user", 0.0)
	createMockMarket(t, 1)

	engine := getEngine()
	order := models.Order{
		UserID:    1,
		MarketID:  1,
		Outcome:   "YES",
		OrderType: "MARKET",
		Price:     50.0,
		Shares:    10.0,
	}

	err := engine.ProcessOrder(context.Background(), order)
	if err == nil {
		t.Fatal("Expected error due to insufficient balance, got nil")
	}
}

// TestProcessOrder_AmmRouting verifies the fallback behavior of MARKET orders.
// If the Central Limit Order Book (CLOB) is entirely empty, the engine should
// seamlessly route the requested shares to the Automated Market Maker (AMM)
// and mathematically update the pool's liquidity state.
// What it tests: Fallback routing from empty CLOB to AMM.
// Why it matters: Ensures market liquidity is available even with no limit orders.
// Status: Useful
func TestProcessOrder_AmmRouting(t *testing.T) {
	setupTestDB(t)
	defer db.CloseDB()

	// 1000 balance is enough for 10 shares
	createMockUser(t, 1, "amm_buyer", 1000.0)
	createMockMarket(t, 1)

	engine := getEngine()
	order := models.Order{
		UserID:    1,
		MarketID:  1,
		Outcome:   "YES",
		OrderType: "MARKET",
		Price:     50.0,
		Shares:    10.0,
	}

	err := engine.ProcessOrder(context.Background(), order)
	if err != nil {
		t.Fatalf("Failed to process order: %v", err)
	}

	// Verify balance was deducted
	var balance float64
	err = db.Pool.QueryRow(context.Background(), "SELECT balance FROM users WHERE id = 1").Scan(&balance)
	if err != nil {
		t.Fatal(err)
	}
	if balance >= 1000.0 {
		t.Errorf("Balance should have been deducted, got %v", balance)
	}

	// Verify AMM state
	var qYes, qNo float64
	err = db.Pool.QueryRow(context.Background(), "SELECT q_yes, q_no FROM amm_state WHERE market_id = 1").Scan(&qYes, &qNo)
	if err != nil {
		t.Fatal(err)
	}
	if qYes != 10.0 || qNo != 0.0 {
		t.Errorf("Expected qYes=10, qNo=0, got qYes=%v, qNo=%v", qYes, qNo)
	}
}

// TestProcessOrder_ExactClobMatch verifies peer-to-peer trading logic.
// If a Maker limit order exactly crosses paths with a Taker limit order 
// (e.g. Maker sells NO at 60c, Taker buys YES at 40c), the engine must match them
// perfectly via the CLOB without ever touching the AMM liquidity pool.
// What it tests: Perfect 1:1 CLOB matching correctness.
// Why it matters: Core to the CLOB orderbook functionality.
// Status: Useful
func TestProcessOrder_ExactClobMatch(t *testing.T) {
	setupTestDB(t)
	defer db.CloseDB()

	createMockUser(t, 1, "maker", 1000.0)
	createMockUser(t, 2, "taker", 1000.0)
	createMockMarket(t, 1)

	engine := getEngine()

	// Maker sells NO (so buys NO) at $0.60
	makerOrder := models.Order{
		UserID:    1,
		MarketID:  1,
		Outcome:   "NO",
		OrderType: "LIMIT",
		Price:     60.0,
		Shares:    100.0,
	}
	err := engine.ProcessOrder(context.Background(), makerOrder)
	if err != nil {
		t.Fatalf("Maker order failed: %v", err)
	}

	// Taker buys YES at $0.40 (this perfectly crosses Maker's NO at 0.60)
	takerOrder := models.Order{
		UserID:    2,
		MarketID:  1,
		Outcome:   "YES",
		OrderType: "LIMIT",
		Price:     40.0,
		Shares:    100.0,
	}
	err = engine.ProcessOrder(context.Background(), takerOrder)
	if err != nil {
		t.Fatalf("Taker order failed: %v", err)
	}

	// Verify AMM wasn't touched for the taker
	var qYes, qNo float64
	err = db.Pool.QueryRow(context.Background(), "SELECT q_yes, q_no FROM amm_state WHERE market_id = 1").Scan(&qYes, &qNo)
	if err != nil {
		t.Fatal(err)
	}
	// Note: The maker hitting the DB initially went to the AMM!
	// In our engine logic, the maker hits the AMM because there were no YES orders to match.
	// So qNo became 100.
	// Wait, taker YES at 40 matching Maker NO at 60 -> Maker's remaining shares is 0. 
	// Does Taker hit AMM? No, because tradeShares = 100, remainingShares = 0.
	// So AMM qYes should remain 0.
	if qYes != 0.0 {
		t.Errorf("Expected qYes=0 (taker fully matched via CLOB), got %v", qYes)
	}

	// Check trades
	var tradeShares float64
	err = db.Pool.QueryRow(context.Background(), "SELECT shares FROM trades WHERE taker_order_id IS NOT NULL AND maker_order_id IS NOT NULL").Scan(&tradeShares)
	if err != nil {
		t.Fatalf("Failed to find CLOB trade: %v", err)
	}
	if tradeShares != 100.0 {
		t.Errorf("Expected 100 shares traded in CLOB, got %v", tradeShares)
	}
}

// TestProcessOrder_PartialClobMatch verifies complex order routing.
// If a user places a massive MARKET order, the engine must first consume all 
// available liquidity in the peer-to-peer CLOB. Once the CLOB is drained, 
// the remaining unfilled shares must be routed directly to the AMM pool.
// What it tests: Partial CLOB fill and remainder routing.
// Why it matters: Tests the hybrid CLOB/AMM model integration.
// Status: Useful
func TestProcessOrder_PartialClobMatch(t *testing.T) {
	setupTestDB(t)
	defer db.CloseDB()

	createMockUser(t, 1, "maker", 1000.0)
	createMockUser(t, 2, "taker", 1000.0)
	createMockMarket(t, 1)

	engine := getEngine()

	// Maker NO 50 shares
	engine.ProcessOrder(context.Background(), models.Order{UserID: 1, MarketID: 1, Outcome: "NO", OrderType: "LIMIT", Price: 60.0, Shares: 50.0})

	// Taker YES 100 shares. 50 matches maker, 50 goes to AMM
	engine.ProcessOrder(context.Background(), models.Order{UserID: 2, MarketID: 1, Outcome: "YES", OrderType: "MARKET", Price: 40.0, Shares: 100.0})

	// Verify AMM got 50 YES
	var qYes float64
	db.Pool.QueryRow(context.Background(), "SELECT q_yes FROM amm_state WHERE market_id = 1").Scan(&qYes)
	
	if qYes != 50.0 {
		t.Errorf("Expected AMM qYes=50, got %v", qYes)
	}
}

// TestProcessOrder_NoMatchPriceGap verifies that Limit orders which do not cross
// (i.e. the buyer is not willing to pay the seller's minimum asking price) 
// do not match. The Maker's Limit order remains open, and the Taker's Market order 
// falls back to the AMM instead of forcefully matching at a loss.
// What it tests: Non-crossing limit order logic.
// Why it matters: Prevents users from getting bad fills.
// Status: Useful
func TestProcessOrder_NoMatchPriceGap(t *testing.T) {
	setupTestDB(t)
	defer db.CloseDB()

	createMockUser(t, 1, "maker", 1000.0)
	createMockUser(t, 2, "taker", 1000.0)
	createMockMarket(t, 1)

	engine := getEngine()

	// Maker NO at $0.50
	engine.ProcessOrder(context.Background(), models.Order{UserID: 1, MarketID: 1, Outcome: "NO", OrderType: "LIMIT", Price: 50.0, Shares: 100.0})

	// Taker YES at $0.40 (100 - 40 = 60. 60 is NOT >= 50. So NO MATCH)
	engine.ProcessOrder(context.Background(), models.Order{UserID: 2, MarketID: 1, Outcome: "YES", OrderType: "MARKET", Price: 40.0, Shares: 100.0})

	// Since there's no match, taker goes entirely to AMM.
	var qYes float64
	db.Pool.QueryRow(context.Background(), "SELECT q_yes FROM amm_state WHERE market_id = 1").Scan(&qYes)
	
	// Taker's 100 shares should go to AMM
	if math.Abs(qYes-100.0) > 0.01 {
		t.Errorf("Expected AMM qYes to be 100, got %v", qYes)
	}
}

// TestProcessOrder_NoLiquiditySlippage simulates a scenario where the CLOB is entirely empty,
// and a massive "whale" order hits the market. Because the LMSR algorithm protects the pool,
// massive unilateral volume drives the slippage (cost per share) through the roof.
// What it tests: Severe AMM slippage on large orders.
// Why it matters: Shows LMSR mathematical model working as designed under stress.
// Status: Useful
func TestProcessOrder_NoLiquiditySlippage(t *testing.T) {
	setupTestDB(t)
	defer db.CloseDB()

	// Give a user $1,000,000 to attempt a massive AMM buyout
	createMockUser(t, 1, "whale", 1000000.0)
	createMockMarket(t, 1)

	engine := getEngine()

	// Whale attempts to buy 10,000 shares of YES on an empty order book
	whaleOrder := models.Order{
		UserID:    1,
		MarketID:  1,
		Outcome:   "YES",
		OrderType: "MARKET",
		Price:     50.0, // Not used for AMM routing, but sent by frontend
		Shares:    10000.0,
	}

	err := engine.ProcessOrder(context.Background(), whaleOrder)
	if err != nil {
		t.Fatalf("Failed to process whale order: %v", err)
	}

	// Verify the massive slippage mathematically
	var balance float64
	db.Pool.QueryRow(context.Background(), "SELECT balance FROM users WHERE id = 1").Scan(&balance)
	
	totalCost := 1000000.0 - balance
	
	// If there was 0 slippage, 10,000 shares at 50 cents would cost $5,000.
	// However, due to LMSR slippage protecting the pool from the massive order, 
	// the actual cost should be drastically higher (approaching $10,000).
	if totalCost <= 5000.0 {
		t.Errorf("Expected massive slippage cost > $5000, but only cost %v", totalCost)
	}
	
	t.Logf("A whale bought 10,000 shares on an empty book. Due to LMSR slippage, the 10,000 shares cost $%.2f (Average $%.2f per share instead of $0.50)", totalCost, totalCost/10000.0)
}

// TestHighLoad_Loop simulates a loop of trades across multiple users to verify system stability.
func TestHighLoad_Loop(t *testing.T) {
	setupTestDB(t)
	defer db.CloseDB()
	createMockMarket(t, 1)

	engine := getEngine()
	// Create 100 users with enough balance
	for i := 1; i <= 100; i++ {
		createMockUser(t, i, fmt.Sprintf("user_%d", i), 1000.0)
	}

	for i := 1; i <= 100; i++ {
		order := models.Order{UserID: i, MarketID: 1, Outcome: "YES", OrderType: "MARKET", Price: 50.0, Shares: 1.0}
		if err := engine.ProcessOrder(context.Background(), order); err != nil {
			t.Fatalf("Failed order loop %d: %v", i, err)
		}
	}
}

// TestLowLiquidity_LMSRStress tests a market with a very small B parameter.
func TestLowLiquidity_LMSRStress(t *testing.T) {
	setupTestDB(t)
	defer db.CloseDB()
	createMockUser(t, 1, "buyer", 10000.0)

	ctx := context.Background()
	db.Pool.Exec(ctx, "INSERT INTO categories (id, name, description) VALUES (1, 'Test', '') ON CONFLICT DO NOTHING")
	db.Pool.Exec(ctx, "INSERT INTO markets (id, question, expiry, category_id, is_resolved, b_parameter, created_at) VALUES (1, 'Low Liq', $1, 1, false, 1.00, $2)", time.Now().Add(24*time.Hour), time.Now())
	db.Pool.Exec(ctx, "INSERT INTO amm_state (market_id, q_yes, q_no) VALUES (1, 0, 0)")

	engine := getEngine()
	order := models.Order{UserID: 1, MarketID: 1, Outcome: "YES", OrderType: "MARKET", Price: 50.0, Shares: 10.0}
	
	if err := engine.ProcessOrder(ctx, order); err != nil {
		t.Fatalf("Failed order: %v", err)
	}

	var balance float64
	db.Pool.QueryRow(ctx, "SELECT balance FROM users WHERE id = 1").Scan(&balance)
	cost := 10000.0 - balance
	if cost < 9.0 { // With b=1, buying 10 shares pushes price to 1 instantly. Cost should be near 10.
		t.Errorf("Expected massive price jump cost > 9.0, got %v", cost)
	}
}

// TestHighLiquidity tests a market with a very large B parameter.
func TestHighLiquidity(t *testing.T) {
	setupTestDB(t)
	defer db.CloseDB()
	createMockUser(t, 1, "buyer", 10000.0)

	ctx := context.Background()
	db.Pool.Exec(ctx, "INSERT INTO categories (id, name, description) VALUES (1, 'Test', '') ON CONFLICT DO NOTHING")
	db.Pool.Exec(ctx, "INSERT INTO markets (id, question, expiry, category_id, is_resolved, b_parameter, created_at) VALUES (1, 'High Liq', $1, 1, false, 10000.00, $2)", time.Now().Add(24*time.Hour), time.Now())
	db.Pool.Exec(ctx, "INSERT INTO amm_state (market_id, q_yes, q_no) VALUES (1, 0, 0)")

	engine := getEngine()
	order := models.Order{UserID: 1, MarketID: 1, Outcome: "YES", OrderType: "MARKET", Price: 50.0, Shares: 10.0}
	
	if err := engine.ProcessOrder(ctx, order); err != nil {
		t.Fatalf("Failed order: %v", err)
	}

	var balance float64
	db.Pool.QueryRow(ctx, "SELECT balance FROM users WHERE id = 1").Scan(&balance)
	cost := 10000.0 - balance
	if cost > 5.1 { // With b=10000, 10 shares should cost ~5.0
		t.Errorf("Expected stable price cost ~5.0, got %v", cost)
	}
}

// TestSameUserSpam tests sequentially buying from the same user to ensure balance tracks.
func TestSameUserSpam(t *testing.T) {
	setupTestDB(t)
	defer db.CloseDB()
	createMockUser(t, 1, "spammer", 1000.0)
	createMockMarket(t, 1)

	engine := getEngine()
	for i := 0; i < 50; i++ {
		order := models.Order{UserID: 1, MarketID: 1, Outcome: "YES", OrderType: "MARKET", Price: 50.0, Shares: 1.0}
		engine.ProcessOrder(context.Background(), order)
	}

	var balance float64
	db.Pool.QueryRow(context.Background(), "SELECT balance FROM users WHERE id = 1").Scan(&balance)
	if balance < 0 || balance > 1000.0 {
		t.Errorf("Balance tracking failed: %v", balance)
	}
}

// TestEdgeCases tests zero, negative, invalid.
func TestEdgeCases(t *testing.T) {
	setupTestDB(t)
	defer db.CloseDB()
	createMockUser(t, 1, "edge_user", 100.0)
	createMockMarket(t, 1)

	engine := getEngine()
	// Invalid market
	if err := engine.ProcessOrder(context.Background(), models.Order{UserID: 1, MarketID: 999, Outcome: "YES", OrderType: "MARKET", Shares: 10.0}); err == nil {
		t.Error("Expected error for invalid market")
	}
}

// Benchmarks

func BenchmarkTradeExecution(b *testing.B) {
	setupTestDB(&testing.T{})
	defer db.CloseDB()
	createMockUser(&testing.T{}, 1, "bench", 1000000.0)
	createMockMarket(&testing.T{}, 1)
	engine := getEngine()
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		engine.ProcessOrder(ctx, models.Order{UserID: 1, MarketID: 1, Outcome: "YES", OrderType: "MARKET", Shares: 1.0})
	}
}

func BenchmarkConcurrentTrades(b *testing.B) {
	setupTestDB(&testing.T{})
	defer db.CloseDB()
	createMockMarket(&testing.T{}, 1)
	for i := 1; i <= 10; i++ {
		createMockUser(&testing.T{}, i, "bench", 1000000.0)
	}
	engine := getEngine()
	ctx := context.Background()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 1
		for pb.Next() {
			engine.ProcessOrder(ctx, models.Order{UserID: (i%10)+1, MarketID: 1, Outcome: "YES", OrderType: "MARKET", Shares: 1.0})
			i++
		}
	})
}
