package models

import (
	"time"
)

type User struct {
	ID       int     `json:"id"`
	Username string  `json:"username"`
	Balance  float64 `json:"balance"`
}

type Market struct {
	ID              int       `json:"id"`
	Question        string    `json:"question"`
	Expiry          time.Time `json:"expiry"`
	CategoryID      int       `json:"category_id"`
	IsResolved      bool      `json:"is_resolved"`
	ResolvedOutcome *string   `json:"resolved_outcome"`
	BParameter      float64   `json:"b_parameter"`
	CreatedAt       time.Time `json:"created_at"`
}

type AmmState struct {
	ID       int     `json:"id"`
	MarketID int     `json:"market_id"`
	QYes     float64 `json:"q_yes"`
	QNo      float64 `json:"q_no"`
}

type Order struct {
	ID           int       `json:"id"`
	UserID       int       `json:"user_id"`
	MarketID     int       `json:"market_id"`
	Outcome      string    `json:"outcome"`
	OrderType    string    `json:"order_type"`
	Price        float64   `json:"price"`
	Shares       float64   `json:"shares"`
	FilledShares float64   `json:"filled_shares"`
	Status       string    `json:"status"`
	CreatedAt    time.Time `json:"created_at"`
}

type Trade struct {
	ID           int       `json:"id"`
	MarketID     int       `json:"market_id"`
	MakerOrderID *int      `json:"maker_order_id"`
	TakerOrderID int       `json:"taker_order_id"`
	Price        float64   `json:"price"`
	Shares       float64   `json:"shares"`
	ExecutedAt   time.Time `json:"executed_at"`
}
