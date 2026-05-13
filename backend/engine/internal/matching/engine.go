package matching

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"time"

	"omnimarket-engine/internal/amm"
	"omnimarket-engine/internal/db"
	"omnimarket-engine/internal/models"
	"omnimarket-engine/internal/ws"
)

type MatchingEngine struct {
	Hub *ws.Hub
}

func NewMatchingEngine(hub *ws.Hub) *MatchingEngine {
	return &MatchingEngine{Hub: hub}
}

// ProcessOrder is the core matching loop.
func (me *MatchingEngine) ProcessOrder(ctx context.Context, order models.Order) error {
	startTime := time.Now()
	tx, err := db.Pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	// Determine opposite outcome (implicitly used in logic)
	// if order.Outcome == "NO" { ... }

	remainingShares := order.Shares

	// 1. CLOB Matching
	// Find crossing limit orders (Price-Time Priority)
	// For a YES buyer at price P, they match with a NO seller at price 1-P or better.
	// Since order prices in OmniMarket are typically 0-1 (or 0-100), we assume they are 0-100 probabilities.
	// Wait, the schema has `price` Numeric. Assume 0-100.
	// So YES price P corresponds to NO price 100 - P.
	
	// Query opposite side orders
	// For YES, we want NO orders where `price <= 100 - order.Price`.
	// For NO, we want YES orders where `price <= 100 - order.Price`.
	var oppositeOrdersQuery string
	if order.Outcome == "YES" {
		oppositeOrdersQuery = `
			SELECT id, user_id, price, shares, filled_shares 
			FROM orders 
			WHERE market_id = $1 AND outcome = 'NO' AND status = 'OPEN' AND price >= (100 - $2)
			ORDER BY price DESC, created_at ASC 
			FOR UPDATE
		`
	} else {
		oppositeOrdersQuery = `
			SELECT id, user_id, price, shares, filled_shares 
			FROM orders 
			WHERE market_id = $1 AND outcome = 'YES' AND status = 'OPEN' AND price >= (100 - $2)
			ORDER BY price DESC, created_at ASC 
			FOR UPDATE
		`
	}

	rows, err := tx.Query(ctx, oppositeOrdersQuery, order.MarketID, order.Price)
	if err != nil {
		return fmt.Errorf("failed to fetch opposite orders: %w", err)
	}

	type makerOrder struct {
		id           int
		userID       int
		price        float64
		shares       float64
		filledShares float64
	}
	var makers []makerOrder

	for rows.Next() {
		var m makerOrder
		if err := rows.Scan(&m.id, &m.userID, &m.price, &m.shares, &m.filledShares); err != nil {
			rows.Close()
			return err
		}
		makers = append(makers, m)
	}
	rows.Close()

	var tradeIDs []int
	for _, m := range makers {
		if remainingShares <= 0 {
			break
		}

		availableShares := m.shares - m.filledShares
		tradeShares := math.Min(remainingShares, availableShares)

		// Execute trade
		remainingShares -= tradeShares
		m.filledShares += tradeShares

		// Update maker order
		makerStatus := "OPEN"
		if m.filledShares >= m.shares {
			makerStatus = "FILLED"
		}
		_, err := tx.Exec(ctx, "UPDATE orders SET filled_shares = $1, status = $2 WHERE id = $3", m.filledShares, makerStatus, m.id)
		if err != nil {
			return err
		}

		// Insert trade
		takerPrice := 100 - m.price
		// Note: taker order ID isn't known yet, we will update it later or insert taker order first.
		// For MVP, we can insert it with a NULL taker_order_id and update it later.
		var tradeID int
		err = tx.QueryRow(ctx, 
			"INSERT INTO trades (market_id, maker_order_id, price, shares) VALUES ($1, $2, $3, $4) RETURNING id",
			order.MarketID, m.id, takerPrice, tradeShares).Scan(&tradeID)
		if err != nil {
			return err
		}
		tradeIDs = append(tradeIDs, tradeID)
	}

	// 2. Insert/Update the current order
	status := "OPEN"
	if remainingShares == 0 {
		status = "FILLED"
	}

	var insertedOrderID int
	err = tx.QueryRow(ctx, `
		INSERT INTO orders (user_id, market_id, outcome, order_type, price, shares, filled_shares, status) 
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8) RETURNING id
	`, order.UserID, order.MarketID, order.Outcome, order.OrderType, order.Price, order.Shares, order.Shares-remainingShares, status).Scan(&insertedOrderID)
	if err != nil {
		return fmt.Errorf("failed to insert order: %w", err)
	}

	for _, tID := range tradeIDs {
		_, err = tx.Exec(ctx, "UPDATE trades SET taker_order_id = $1 WHERE id = $2", insertedOrderID, tID)
		if err != nil {
			return err
		}
	}

	if len(tradeIDs) > 0 {
		tradedShares := order.Shares - remainingShares
		clobLog := map[string]interface{}{
			"event":              "trade",
			"user_id":            order.UserID,
			"market_id":          order.MarketID,
			"quantity":           tradedShares,
			"engine":             "CLOB",
			"matched":            true,
			"match_count":        len(tradeIDs),
			"remaining_quantity": remainingShares,
			"route":              "CLOB",
			"fallback":           false,
			"db_time":            time.Since(startTime).Milliseconds(),
			"total_time":         time.Since(startTime).Milliseconds(),
		}
		clobJSON, _ := json.Marshal(clobLog)
		log.Println(string(clobJSON))
	}

	// Wait, if it's a MARKET order or allows AMM fallback, we route the remaining to AMM.
	if remainingShares > 0 && order.OrderType == "MARKET" {
		// Fetch AMM State with row-level locking
		var qYes, qNo float64
		err = tx.QueryRow(ctx, "SELECT q_yes, q_no FROM amm_state WHERE market_id = $1 FOR UPDATE", order.MarketID).Scan(&qYes, &qNo)
		if err != nil {
			return fmt.Errorf("failed to fetch amm state: %w", err)
		}

		// We need the market's B parameter
		var bParameter float64
		err = tx.QueryRow(ctx, "SELECT b_parameter FROM markets WHERE id = $1", order.MarketID).Scan(&bParameter)
		if err != nil {
			return fmt.Errorf("failed to fetch market: %w", err)
		}

		isYes := order.Outcome == "YES"
		cost := amm.CalculateCostForShares(qYes, qNo, bParameter, remainingShares, isYes)

		// Determine if user balance covers the cost
		var balance float64
		err = tx.QueryRow(ctx, "SELECT balance FROM users WHERE id = $1 FOR UPDATE", order.UserID).Scan(&balance)
		if err != nil {
			return fmt.Errorf("failed to fetch user balance: %w", err)
		}

		if balance < cost {
			return fmt.Errorf("insufficient balance: cost %f, balance %f", cost, balance)
		}

		// Deduct balance
		_, err = tx.Exec(ctx, "UPDATE users SET balance = balance - $1 WHERE id = $2", cost, order.UserID)
		if err != nil {
			return fmt.Errorf("failed to deduct balance: %w", err)
		}

		// Update AMM State
		if isYes {
			qYes += remainingShares
		} else {
			qNo += remainingShares
		}
		_, err = tx.Exec(ctx, "UPDATE amm_state SET q_yes = $1, q_no = $2 WHERE market_id = $3", qYes, qNo, order.MarketID)
		if err != nil {
			return fmt.Errorf("failed to update amm state: %w", err)
		}

		// Mark order as filled
		_, err = tx.Exec(ctx, "UPDATE orders SET filled_shares = shares, status = 'FILLED' WHERE id = $1", insertedOrderID)
		if err != nil {
			return err
		}

		// Insert AMM Trade (maker_order_id = null)
		avgPrice := cost / remainingShares
		_, err = tx.Exec(ctx, 
			"INSERT INTO trades (market_id, taker_order_id, price, shares) VALUES ($1, $2, $3, $4)",
			order.MarketID, insertedOrderID, avgPrice, remainingShares)
		if err != nil {
			return err
		}
		
		// Broadcast new AMM price
		newPriceYes := amm.CalculatePrice(qYes, qNo, bParameter, true)

		var priceBefore float64
		var priceAfter float64
		if isYes {
			priceBefore = amm.CalculatePrice(qYes-remainingShares, qNo, bParameter, true)
			priceAfter = newPriceYes
		} else {
			priceBefore = amm.CalculatePrice(qYes, qNo-remainingShares, bParameter, false)
			priceAfter = amm.CalculatePrice(qYes, qNo, bParameter, false)
		}

		lmsrLog := map[string]interface{}{
			"event":        "trade",
			"user_id":      order.UserID,
			"market_id":    order.MarketID,
			"quantity":     remainingShares,
			"engine":       "LMSR",
			"price_before": priceBefore,
			"price_after":  priceAfter,
			"route":        "LMSR",
			"fallback":     true,
			"db_time":      time.Since(startTime).Milliseconds(),
			"total_time":   time.Since(startTime).Milliseconds(),
		}
		lmsrJSON, _ := json.Marshal(lmsrLog)
		log.Println(string(lmsrJSON))

		me.Hub.Broadcast <- ws.Message{
			MarketID: order.MarketID,
			Type:     "AMM_PRICE_UPDATE",
			Data:     map[string]float64{"price_yes": newPriceYes, "price_no": 1 - newPriceYes},
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("failed to commit tx: %w", err)
	}

	// Broadcast orderbook update or trade execution
	me.Hub.Broadcast <- ws.Message{
		MarketID: order.MarketID,
		Type:     "TRADE_EXECUTED",
		Data:     order, // Send basic order details
	}

	return nil
}
