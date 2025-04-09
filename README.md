# 🔐 Ledger Service

A RESTful API service for managing customer accounts and transactions with multi-currency support.

## 🚀 Features

- ✅ Create customer accounts with initial balance
- ✅ Process credit and debit transactions
- ✅ View current balance
- ✅ View transaction history with pagination
- ✅ Concurrent transaction safety
- ✅ PostgreSQL database for persistence
- ✅ Swagger/OpenAPI documentation
- ✅ Docker support
- ✅ Railway deployment
- ✅ Multi-currency support (USD, EUR, GBP)
- ✅ Real-time currency conversion using ExchangeRate-API

## 🌐 Live Demo

The service is hosted on Railway:
- API: https://ledger-service-production.up.railway.app
- Swagger UI: https://ledger-service-production.up.railway.app/swagger/index.html

## 📚 API Documentation

### 1. Create Customer Account
```bash
POST /customers

Request:
{
  "name": "John Doe",
  "initial_balance": 1000
}

Response:
{
  "customer_id": "550e8400-e29b-41d4-a716-446655440000",
  "name": "John Doe",
  "balance": 1000
}
```

### 2. Create Transaction
```bash
POST /transactions

Request:
{
  "customer_id": "550e8400-e29b-41d4-a716-446655440000",
  "type": "credit",  # or "debit"
  "amount": 200
}

Response:
{
  "transaction_id": "550e8400-e29b-41d4-a716-446655440000",
  "status": "success",
  "balance": 1200
}
```

### 3. Get Current Balance
```bash
GET /customers/{customer_id}/balance?currency=EUR

Response:
{
  "customer_id": "550e8400-e29b-41d4-a716-446655440000",
  "balance": 1104.00,  # Converted from USD to EUR
  "currency": "EUR"
}
```

### 4. Get Transaction History (with Pagination)
```bash
GET /customers/{customer_id}/transactions?page=1&page_size=10

Response:
[
  {
    "transaction_id": "550e8400-e29b-41d4-a716-446655440000",
    "type": "credit",
    "amount": 200,
    "timestamp": "2025-04-08T17:09:17Z"
  }
]
```

## 🛠️ Local Development

### Prerequisites
- Docker and Docker Compose
- Git

### Quick Start

1. Clone the repository:
```bash
git clone https://github.com/arjunbalu1/ledger-service.git
cd ledger-service
```

2. Start the application and database:
```bash
docker compose up --build
```

3. Run database migrations:
```bash
docker exec -i ledger-service-main-db-1 psql -U ledger -d ledger_db < migrations/init.sql
```

The service will be available at:
- API: http://localhost:8080
- Swagger UI: http://localhost:8080/swagger/index.html

## 🧪 Testing

### Automated Tests
```bash
go test ./...
```

### Manual Testing

#### 1. Create Customer Account
```http
POST http://localhost:8080/customers
Content-Type: application/json

{
  "name": "John Doe",
  "initial_balance": 1000
}
```

#### 2. Create Transaction
```http
POST http://localhost:8080/transactions
Content-Type: application/json

{
  "customer_id": "YOUR_CUSTOMER_ID",
  "type": "credit",
  "amount": 200
}
```

#### 3. Get Current Balance
```http
GET http://localhost:8080/customers/{customer_id}/balance?currency=EUR
```

#### 4. View Transactions
```http
GET http://localhost:8080/customers/{customer_id}/transactions?page=1&page_size=10
```

## 🔒 Security Features

- Database credentials managed through environment variables
- Input validation for all API endpoints
- Concurrent transaction safety using database transactions
- Row-level locking for balance updates

## 🏗️ Architecture

- **Language**: Go
- **Framework**: Gin
- **Database**: PostgreSQL
- **ORM**: pgx
- **Containerization**: Docker
- **Deployment**: Railway
- **Documentation**: Swagger/OpenAPI
- **Currency Conversion**: ExchangeRate-API

## 📝 Notes

- The service uses UUIDs for customer and transaction IDs
- All monetary values are stored as float64
- Transactions are atomic and concurrent-safe
- Pagination is supported for transaction history
- Health check endpoint available at `/health`
