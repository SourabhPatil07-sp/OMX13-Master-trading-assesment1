package api

import (
	"context"
	"net/http"
	"omnimarket-engine/internal/matching"
	"omnimarket-engine/internal/models"
	"omnimarket-engine/internal/ws"

	"github.com/gin-gonic/gin"
)

type Router struct {
	Engine *matching.MatchingEngine
	Hub    *ws.Hub
}

func NewRouter(engine *matching.MatchingEngine, hub *ws.Hub) *Router {
	return &Router{Engine: engine, Hub: hub}
}

func CORSMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, accept, origin, Cache-Control, X-Requested-With")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS, GET, PUT")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	}
}

func (r *Router) SetupRoutes() *gin.Engine {
	app := gin.Default()
	app.Use(CORSMiddleware())

	app.POST("/api/orders", r.PlaceOrder)
	app.GET("/ws", func(c *gin.Context) {
		ws.ServeWs(r.Hub, c.Writer, c.Request)
	})

	return app
}

type PlaceOrderRequest struct {
	UserID    int     `json:"user_id" binding:"required"`
	MarketID  int     `json:"market_id" binding:"required"`
	Outcome   string  `json:"outcome" binding:"required"`
	OrderType string  `json:"order_type" binding:"required"`
	Price     float64 `json:"price" binding:"required"`
	Shares    float64 `json:"shares" binding:"required"`
}

func (r *Router) PlaceOrder(c *gin.Context) {
	var req PlaceOrderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	order := models.Order{
		UserID:    req.UserID,
		MarketID:  req.MarketID,
		Outcome:   req.Outcome,
		OrderType: req.OrderType,
		Price:     req.Price,
		Shares:    req.Shares,
	}

	err := r.Engine.ProcessOrder(context.Background(), order)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Order processed successfully"})
}
