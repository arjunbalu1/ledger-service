package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	pgxmock "github.com/pashagolub/pgxmock/v3"
	"github.com/stretchr/testify/assert"
)

var mock pgxmock.PgxConnIface

func setupTestRouter() (*gin.Engine, error) {
	var err error
	mock, err = pgxmock.NewConn()
	if err != nil {
		return nil, err
	}

	gin.SetMode(gin.TestMode)
	r := gin.Default()
	InitDB(mock)
	return r, nil
}

func TestCreateCustomer(t *testing.T) {
	router, err := setupTestRouter()
	if err != nil {
		t.Fatalf("Failed to setup test router: %v", err)
	}
	defer mock.Close(context.Background())

	router.POST("/customers", CreateCustomer)

	tests := []struct {
		name       string
		payload    map[string]interface{}
		wantStatus int
		wantErr    bool
		setupMock  func()
	}{
		{
			name: "valid customer",
			payload: map[string]interface{}{
				"name":            "John Doe",
				"initial_balance": 1000,
			},
			wantStatus: http.StatusCreated,
			wantErr:    false,
			setupMock: func() {
				mock.ExpectExec(`INSERT INTO customers \(id, name, balance\) VALUES \(\$1, \$2, \$3\)`).
					WithArgs(pgxmock.AnyArg(), "John Doe", float64(1000)).
					WillReturnResult(pgxmock.NewResult("INSERT", 1))
			},
		},
		{
			name: "negative balance",
			payload: map[string]interface{}{
				"name":            "John Doe",
				"initial_balance": -1000,
			},
			wantStatus: http.StatusBadRequest,
			wantErr:    true,
			setupMock:  func() {},
		},
		{
			name: "missing name",
			payload: map[string]interface{}{
				"initial_balance": 1000,
			},
			wantStatus: http.StatusBadRequest,
			wantErr:    true,
			setupMock:  func() {},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setupMock()
			jsonBytes, _ := json.Marshal(tt.payload)
			req := httptest.NewRequest("POST", "/customers", bytes.NewBuffer(jsonBytes))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.wantStatus, w.Code)
			if !tt.wantErr {
				var response map[string]interface{}
				err := json.Unmarshal(w.Body.Bytes(), &response)
				assert.NoError(t, err)
				assert.Contains(t, response, "customer_id")
				assert.Contains(t, response, "name")
				assert.Contains(t, response, "balance")
			}
		})
	}
}

func TestCreateTransaction(t *testing.T) {
	router, err := setupTestRouter()
	if err != nil {
		t.Fatalf("Failed to setup test router: %v", err)
	}
	defer mock.Close(context.Background())

	router.POST("/transactions", CreateTransaction)

	customerID := uuid.New()
	tests := []struct {
		name       string
		payload    map[string]interface{}
		wantStatus int
		wantErr    bool
		setupMock  func()
	}{
		{
			name: "valid credit transaction",
			payload: map[string]interface{}{
				"customer_id": customerID,
				"type":        "credit",
				"amount":      200,
			},
			wantStatus: http.StatusCreated,
			wantErr:    false,
			setupMock: func() {
				mock.ExpectBegin()
				mock.ExpectQuery(`SELECT balance FROM customers WHERE id = \$1 FOR UPDATE`).
					WithArgs(customerID).
					WillReturnRows(pgxmock.NewRows([]string{"balance"}).AddRow(float64(1000)))
				mock.ExpectExec(`UPDATE customers SET balance = \$1 WHERE id = \$2`).
					WithArgs(float64(1200), customerID).
					WillReturnResult(pgxmock.NewResult("UPDATE", 1))
				mock.ExpectExec(`INSERT INTO transactions \(id, customer_id, type, amount\) VALUES \(\$1, \$2, \$3, \$4\)`).
					WithArgs(pgxmock.AnyArg(), customerID, "credit", float64(200)).
					WillReturnResult(pgxmock.NewResult("INSERT", 1))
				mock.ExpectCommit()
			},
		},
		{
			name: "invalid transaction type",
			payload: map[string]interface{}{
				"customer_id": customerID,
				"type":        "invalid",
				"amount":      200,
			},
			wantStatus: http.StatusBadRequest,
			wantErr:    true,
			setupMock:  func() {},
		},
		{
			name: "negative amount",
			payload: map[string]interface{}{
				"customer_id": customerID,
				"type":        "credit",
				"amount":      -200,
			},
			wantStatus: http.StatusBadRequest,
			wantErr:    true,
			setupMock:  func() {},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setupMock()
			jsonBytes, _ := json.Marshal(tt.payload)
			req := httptest.NewRequest("POST", "/transactions", bytes.NewBuffer(jsonBytes))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.wantStatus, w.Code)
			if !tt.wantErr {
				var response map[string]interface{}
				err := json.Unmarshal(w.Body.Bytes(), &response)
				assert.NoError(t, err)
				assert.Contains(t, response, "transaction_id")
				assert.Contains(t, response, "status")
				assert.Contains(t, response, "balance")
			}
		})
	}
}

func TestGetBalance(t *testing.T) {
	router, err := setupTestRouter()
	if err != nil {
		t.Fatalf("Failed to setup test router: %v", err)
	}
	defer mock.Close(context.Background())

	router.GET("/customers/:customer_id/balance", GetBalance)

	customerID := uuid.New()
	tests := []struct {
		name       string
		customerID uuid.UUID
		wantStatus int
		wantErr    bool
		setupMock  func()
	}{
		{
			name:       "existing customer",
			customerID: customerID,
			wantStatus: http.StatusOK,
			wantErr:    false,
			setupMock: func() {
				mock.ExpectQuery(`SELECT balance FROM customers WHERE id = \$1`).
					WithArgs(customerID).
					WillReturnRows(pgxmock.NewRows([]string{"balance"}).AddRow(float64(1000)))
			},
		},
		{
			name:       "non-existent customer",
			customerID: uuid.New(),
			wantStatus: http.StatusNotFound,
			wantErr:    true,
			setupMock: func() {
				mock.ExpectQuery(`SELECT balance FROM customers WHERE id = \$1`).
					WithArgs(pgxmock.AnyArg()).
					WillReturnError(pgx.ErrNoRows)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setupMock()
			req := httptest.NewRequest("GET", "/customers/"+tt.customerID.String()+"/balance", nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.wantStatus, w.Code)
			if !tt.wantErr {
				var response map[string]interface{}
				err := json.Unmarshal(w.Body.Bytes(), &response)
				assert.NoError(t, err)
				assert.Contains(t, response, "customer_id")
				assert.Contains(t, response, "balance")
			}
		})
	}
}

func TestGetTransactions(t *testing.T) {
	router, err := setupTestRouter()
	if err != nil {
		t.Fatalf("Failed to setup test router: %v", err)
	}
	defer mock.Close(context.Background())

	router.GET("/customers/:customer_id/transactions", GetTransactions)

	customerID := uuid.New()
	transactionID := uuid.New()
	timestampTime := time.Now().UTC()
	timestamp := timestampTime.Format(time.RFC3339)

	tests := []struct {
		name       string
		customerID uuid.UUID
		wantStatus int
		wantErr    bool
		setupMock  func()
	}{
		{
			name:       "existing customer with transactions",
			customerID: customerID,
			wantStatus: http.StatusOK,
			wantErr:    false,
			setupMock: func() {
				mock.ExpectQuery(`SELECT EXISTS\(SELECT 1 FROM customers WHERE id = \$1\)`).
					WithArgs(customerID).
					WillReturnRows(pgxmock.NewRows([]string{"exists"}).AddRow(true))

				mock.ExpectQuery(`SELECT COUNT\(\*\) FROM transactions WHERE customer_id = \$1`).
					WithArgs(customerID).
					WillReturnRows(pgxmock.NewRows([]string{"count"}).AddRow(1))

				mock.ExpectQuery(`SELECT id, type, amount, created_at FROM transactions WHERE customer_id = \$1 ORDER BY created_at DESC LIMIT \$2 OFFSET \$3`).
					WithArgs(customerID, 10, 0).
					WillReturnRows(pgxmock.NewRows([]string{"id", "type", "amount", "created_at"}).
						AddRow(transactionID, "credit", float64(100), timestampTime))
			},
		},
		{
			name:       "non-existent customer",
			customerID: uuid.New(),
			wantStatus: http.StatusNotFound,
			wantErr:    true,
			setupMock: func() {
				mock.ExpectQuery(`SELECT EXISTS\(SELECT 1 FROM customers WHERE id = \$1\)`).
					WithArgs(pgxmock.AnyArg()).
					WillReturnRows(pgxmock.NewRows([]string{"exists"}).AddRow(false))
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setupMock()
			req := httptest.NewRequest("GET", "/customers/"+tt.customerID.String()+"/transactions", nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.wantStatus, w.Code)

			if !tt.wantErr {
				var transactions []Transaction
				err := json.Unmarshal(w.Body.Bytes(), &transactions)
				if assert.NoError(t, err) {
					assert.Len(t, transactions, 1)
					tx := transactions[0]
					assert.Equal(t, transactionID, tx.ID)
					assert.Equal(t, "credit", tx.Type)
					assert.Equal(t, float64(100), tx.Amount)
					assert.Equal(t, timestamp, tx.Timestamp)

					// Verify pagination headers
					assert.Equal(t, "1", w.Header().Get("X-Total-Count"))
					assert.Equal(t, "1", w.Header().Get("X-Page"))
					assert.Equal(t, "10", w.Header().Get("X-Page-Size"))
					assert.Equal(t, "1", w.Header().Get("X-Total-Pages"))
				}
			}
		})
	}
}
