package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// Customer represents a customer account
// @Description Customer account information
type Customer struct {
	ID             uuid.UUID `json:"customer_id" example:"550e8400-e29b-41d4-a716-446655440000" format:"uuid"`
	Name           string    `json:"name" binding:"required" example:"John Doe" minLength:"1" maxLength:"255"`
	Balance        float64   `json:"balance" example:"1000" minimum:"0"`
	InitialBalance float64   `json:"initial_balance" example:"1000" minimum:"0"`
}

// Transaction represents a financial transaction
// @Description Financial transaction information
type Transaction struct {
	ID         uuid.UUID `json:"transaction_id" example:"550e8400-e29b-41d4-a716-446655440000" format:"uuid"`
	CustomerID uuid.UUID `json:"customer_id" example:"550e8400-e29b-41d4-a716-446655440000" format:"uuid"`
	Type       string    `json:"type" binding:"required,oneof=credit debit" example:"credit" enums:"credit,debit"`
	Amount     float64   `json:"amount" binding:"required,gt=0" example:"200" minimum:"0.01"`
	Timestamp  string    `json:"timestamp,omitempty" example:"2025-04-08T17:09:17Z" format:"date-time"`
}

// CustomerResponse represents the response for customer operations
type CustomerResponse struct {
	CustomerID uuid.UUID `json:"customer_id" example:"550e8400-e29b-41d4-a716-446655440000" format:"uuid"`
	Name       string    `json:"name" example:"John Doe"`
	Balance    float64   `json:"balance" example:"1000"`
}

// TransactionResponse represents the response for transaction operations
type TransactionResponse struct {
	TransactionID uuid.UUID `json:"transaction_id" example:"550e8400-e29b-41d4-a716-446655440000" format:"uuid"`
	Status        string    `json:"status" example:"success" enums:"success"`
	Balance       float64   `json:"balance" example:"800"`
}

// BalanceResponse represents the response for balance operations
type BalanceResponse struct {
	CustomerID uuid.UUID `json:"customer_id" example:"550e8400-e29b-41d4-a716-446655440000" format:"uuid"`
	Balance    float64   `json:"balance" example:"800"`
}

// ErrorResponse represents an error response
type ErrorResponse struct {
	Error string `json:"error" example:"Invalid input"`
}

type DBConn interface {
	Begin(ctx context.Context) (pgx.Tx, error)
	Exec(ctx context.Context, sql string, arguments ...interface{}) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...interface{}) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...interface{}) pgx.Row
}

var (
	db DBConn
)

func InitDB(conn DBConn) error {
	db = conn
	return nil
}

// @Summary Create a new customer account
// @Description Create a new customer account with initial balance
// @Tags customers
// @Accept json
// @Produce json
// @Param customer body Customer true "Customer information"
// @Success 201 {object} CustomerResponse "Customer created successfully"
// @Failure 400 {object} ErrorResponse "Invalid input data"
// @Failure 500 {object} ErrorResponse "Internal server error"
// @Router /customers [post]
func CreateCustomer(c *gin.Context) {
	var customer Customer
	if err := c.ShouldBindJSON(&customer); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Invalid input: Name is required and balance must be non-negative"})
		return
	}

	// Use initial_balance if provided, otherwise use balance
	balance := customer.InitialBalance
	if balance == 0 {
		balance = customer.Balance
	}

	if balance < 0 {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Invalid input: Balance must be non-negative"})
		return
	}

	customer.ID = uuid.New()
	customer.Balance = balance

	// Insert customer into database
	_, err := db.Exec(c.Request.Context(),
		"INSERT INTO customers (id, name, balance) VALUES ($1, $2, $3)",
		customer.ID, customer.Name, customer.Balance)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "Failed to create customer"})
		return
	}

	c.JSON(http.StatusCreated, CustomerResponse{
		CustomerID: customer.ID,
		Name:       customer.Name,
		Balance:    customer.Balance,
	})
}

// @Summary Create a new transaction
// @Description Create a new credit or debit transaction for a customer
// @Tags transactions
// @Accept json
// @Produce json
// @Param transaction body Transaction true "Transaction information"
// @Success 201 {object} TransactionResponse "Transaction processed successfully"
// @Failure 400 {object} ErrorResponse "Invalid input data or insufficient balance"
// @Failure 404 {object} ErrorResponse "Customer not found"
// @Failure 500 {object} ErrorResponse "Internal server error"
// @Router /transactions [post]
func CreateTransaction(c *gin.Context) {
	var transaction Transaction
	if err := c.ShouldBindJSON(&transaction); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Invalid input: customer_id, type (credit/debit), and amount (> 0) are required"})
		return
	}

	// Start transaction
	tx, err := db.Begin(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "Failed to start transaction"})
		return
	}
	defer tx.Rollback(c.Request.Context())

	// Get current balance with row lock
	var currentBalance float64
	err = tx.QueryRow(c.Request.Context(),
		"SELECT balance FROM customers WHERE id = $1 FOR UPDATE",
		transaction.CustomerID).Scan(&currentBalance)
	if err != nil {
		if err == pgx.ErrNoRows {
			c.JSON(http.StatusNotFound, ErrorResponse{Error: "Customer not found"})
		} else {
			c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "Failed to get current balance"})
		}
		return
	}

	// Calculate new balance
	var newBalance float64
	if transaction.Type == "debit" {
		if currentBalance < transaction.Amount {
			c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Insufficient balance"})
			return
		}
		newBalance = currentBalance - transaction.Amount
	} else {
		newBalance = currentBalance + transaction.Amount
	}

	// Update customer balance
	_, err = tx.Exec(c.Request.Context(),
		"UPDATE customers SET balance = $1 WHERE id = $2",
		newBalance, transaction.CustomerID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "Failed to update balance"})
		return
	}

	// Insert transaction
	transaction.ID = uuid.New()
	_, err = tx.Exec(c.Request.Context(),
		"INSERT INTO transactions (id, customer_id, type, amount) VALUES ($1, $2, $3, $4)",
		transaction.ID, transaction.CustomerID, transaction.Type, transaction.Amount)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "Failed to create transaction"})
		return
	}

	// Commit transaction
	if err := tx.Commit(c.Request.Context()); err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "Failed to commit transaction"})
		return
	}

	c.JSON(http.StatusCreated, TransactionResponse{
		TransactionID: transaction.ID,
		Status:        "success",
		Balance:       newBalance,
	})
}

// Helper function to validate currency codes
func isValidCurrency(currency string) bool {
	validCurrencies := map[string]bool{
		"USD": true,
		"EUR": true,
		"GBP": true,
	}
	return validCurrencies[currency]
}

// Function to get exchange rate
func getExchangeRate(fromCurrency, toCurrency string, amount float64) (float64, error) {
	apiKey := "1a7b5574bdb95f1770750778"
	url := fmt.Sprintf("https://v6.exchangerate-api.com/v6/%s/pair/%s/%s/%.2f",
		apiKey, fromCurrency, toCurrency, amount)

	resp, err := http.Get(url)
	if err != nil {
		return 0, fmt.Errorf("failed to call exchange rate API: %v", err)
	}
	defer resp.Body.Close()

	var result struct {
		Result           string  `json:"result"`
		Documentation    string  `json:"documentation"`
		TermsOfUse       string  `json:"terms_of_use"`
		TimeLastUpdate   int64   `json:"time_last_update_unix"`
		TimeNextUpdate   int64   `json:"time_next_update_unix"`
		BaseCode         string  `json:"base_code"`
		TargetCode       string  `json:"target_code"`
		ConversionRate   float64 `json:"conversion_rate"`
		ConversionResult float64 `json:"conversion_result,omitempty"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return 0, fmt.Errorf("failed to decode exchange rate response: %v", err)
	}

	if result.Result != "success" {
		return 0, fmt.Errorf("exchange rate API error: %s", result.Result)
	}

	return result.ConversionResult, nil
}

// GetBalance returns the current balance for a customer
func GetBalance(c *gin.Context) {
	customerID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Invalid customer ID"})
		return
	}

	// Get target currency from query parameter
	targetCurrency := c.DefaultQuery("currency", "USD")
	if !isValidCurrency(targetCurrency) {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Invalid currency code"})
		return
	}

	// Get customer's current balance
	var currentBalance float64
	err = db.QueryRow(context.Background(),
		"SELECT balance FROM customers WHERE id = $1",
		customerID).Scan(&currentBalance)

	if err != nil {
		if err == pgx.ErrNoRows {
			c.JSON(http.StatusNotFound, ErrorResponse{Error: "Customer not found"})
		} else {
			c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "Database error"})
		}
		return
	}

	// Convert balance using Forex API
	convertedBalance, err := getExchangeRate("USD", targetCurrency, currentBalance)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"customer_id": customerID,
		"balance":     convertedBalance,
		"currency":    targetCurrency,
	})
}

// @Summary Get transaction history
// @Description Get paginated transaction history for a customer
// @Tags transactions
// @Produce json
// @Param customer_id path string true "Customer ID" format(uuid)
// @Param page query int false "Page number (1-based)" minimum(1) default(1)
// @Param page_size query int false "Number of items per page" minimum(1) maximum(100) default(10)
// @Success 200 {array} Transaction "List of transactions"
// @Failure 400 {object} ErrorResponse "Invalid customer ID format or pagination parameters"
// @Failure 404 {object} ErrorResponse "Customer not found"
// @Failure 500 {object} ErrorResponse "Internal server error"
// @Header 200 {string} X-Total-Count "Total number of transactions"
// @Header 200 {string} X-Page "Current page number"
// @Header 200 {string} X-Page-Size "Items per page"
// @Header 200 {string} X-Total-Pages "Total number of pages"
// @Router /customers/{customer_id}/transactions [get]
func GetTransactions(c *gin.Context) {
	customerID, err := uuid.Parse(c.Param("customer_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid customer ID"})
		return
	}

	// Get pagination parameters with defaults
	page := 1
	pageSize := 10
	if p := c.Query("page"); p != "" {
		if _, err := fmt.Sscanf(p, "%d", &page); err != nil || page < 1 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid page number"})
			return
		}
	}
	if ps := c.Query("page_size"); ps != "" {
		if _, err := fmt.Sscanf(ps, "%d", &pageSize); err != nil || pageSize < 1 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid page size"})
			return
		}
	}

	// Verify customer exists
	var exists bool
	err = db.QueryRow(c.Request.Context(),
		"SELECT EXISTS(SELECT 1 FROM customers WHERE id = $1)",
		customerID).Scan(&exists)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to verify customer"})
		return
	}
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "Customer not found"})
		return
	}

	// Get total count
	var totalCount int
	err = db.QueryRow(c.Request.Context(),
		"SELECT COUNT(*) FROM transactions WHERE customer_id = $1",
		customerID).Scan(&totalCount)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get total count"})
		return
	}

	// Calculate offset
	offset := (page - 1) * pageSize

	rows, err := db.Query(c.Request.Context(),
		"SELECT id, type, amount, created_at FROM transactions WHERE customer_id = $1 ORDER BY created_at DESC LIMIT $2 OFFSET $3",
		customerID, pageSize, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch transactions"})
		return
	}
	defer rows.Close()

	var transactions []gin.H
	for rows.Next() {
		var id uuid.UUID
		var txType string
		var amount float64
		var timestamp time.Time
		err := rows.Scan(&id, &txType, &amount, &timestamp)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to scan transaction"})
			return
		}

		transactions = append(transactions, gin.H{
			"transaction_id": id,
			"type":           txType,
			"amount":         amount,
			"timestamp":      timestamp.Format(time.RFC3339),
		})
	}

	// Add pagination metadata in headers
	c.Header("X-Total-Count", fmt.Sprintf("%d", totalCount))
	c.Header("X-Page", fmt.Sprintf("%d", page))
	c.Header("X-Page-Size", fmt.Sprintf("%d", pageSize))
	c.Header("X-Total-Pages", fmt.Sprintf("%d", (totalCount+pageSize-1)/pageSize))

	c.JSON(http.StatusOK, transactions)
}
