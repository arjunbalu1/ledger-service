package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"ledger-service/handlers"

	_ "ledger-service/docs" // Import generated docs

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

// @title           Ledger Service API
// @version         1.0
// @description     A simple ledger service that maintains customer balances and transactions.
// @description     This service provides endpoints for managing customer accounts and processing financial transactions.
// @host           ledger-service-production.up.railway.app
// @BasePath       /
// @schemes        https
// @produce        json
// @consumes       json
// @contact.name   API Support
// @contact.email  support@example.com
// @license.name   MIT
// @license.url    https://opensource.org/licenses/MIT
// @tag.name       customers
// @tag.description Operations about customers
// @tag.name       transactions
// @tag.description Operations about transactions

func main() {
	// Get configuration from environment variables
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		log.Fatal("DATABASE_URL environment variable is required")
	}
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	// Initialize database connection
	conn, err := pgx.Connect(context.Background(), dbURL)
	if err != nil {
		log.Fatalf("Unable to connect to database: %v\n", err)
	}
	defer conn.Close(context.Background())

	// Verify database connection
	if err := conn.Ping(context.Background()); err != nil {
		log.Fatalf("Unable to ping database: %v\n", err)
	}

	// Initialize handlers with database connection
	handlers.InitDB(conn)

	// Initialize Gin router
	router := gin.Default()

	// Add CORS middleware
	router.Use(cors.Default())

	// Add request logging middleware
	router.Use(gin.Logger())

	// Root path handler (for Railway healthcheck)
	router.GET("/", func(c *gin.Context) {
		// Check database connection
		if err := conn.Ping(context.Background()); err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{
				"status": "unhealthy",
				"error":  "database connection failed",
			})
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"status":  "healthy",
			"service": "ledger-service",
			"version": "1.0",
		})
	})

	// Health check endpoint
	router.GET("/health", func(c *gin.Context) {
		// Check database connection
		if err := conn.Ping(context.Background()); err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{
				"status":  "unhealthy",
				"error":   "database connection failed",
				"service": "ledger-service",
				"version": "1.0",
			})
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"status":  "healthy",
			"service": "ledger-service",
			"version": "1.0",
		})
	})

	// Setup routes
	router.POST("/customers", handlers.CreateCustomer)
	router.POST("/transactions", handlers.CreateTransaction)
	router.GET("/customers/:customer_id/balance", handlers.GetBalance)
	router.GET("/customers/:customer_id/transactions", handlers.GetTransactions)

	// Swagger documentation
	url := ginSwagger.URL("/swagger/doc.json") // The url pointing to API definition
	router.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler, url))

	// Create HTTP server with timeouts
	srv := &http.Server{
		Addr:         ":" + port,
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Start server in a goroutine
	go func() {
		log.Printf("Server starting on port %s\n", port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen: %s\n", err)
		}
	}()

	// Wait for interrupt signal to gracefully shutdown the server
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("Shutting down server...")

	// Create a deadline for server shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Printf("Server forced to shutdown: %v", err)
	}
}
