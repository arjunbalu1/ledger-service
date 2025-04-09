# Ledger Service

A RESTful service that maintains customer balances and processes transactions. This service provides a simple backend financial system for managing customer accounts and transactions.

## Features

- Create customer accounts with initial balance
- Process credit and debit transactions
- View current balance
- View transaction history with pagination
- Concurrent transaction safety
- PostgreSQL database for persistence
- Swagger/OpenAPI documentation
- Docker support

## Hosting

The service is hosted on Railway, with the application running in a container:
- Application: https://ledger-service-production.up.railway.app
- Database: PostgreSQL instance on Railway

## API Documentation

The API documentation is available at:
- Swagger UI: https://ledger-service-production.up.railway.app/swagger/index.html
- OpenAPI Spec: https://ledger-service-production.up.railway.app/swagger/doc.json

## API Endpoints

### 1. Create Customer Account
```bash
POST https://ledger-service-production.up.railway.app/customers

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
POST https://ledger-service-production.up.railway.app/transactions

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
GET https://ledger-service-production.up.railway.app/customers/{customer_id}/balance

Response:
{
  "customer_id": "550e8400-e29b-41d4-a716-446655440000",
  "balance": 1200
}
```

### 4. Get Transaction History (with Pagination)
```bash
GET https://ledger-service-production.up.railway.app/customers/{customer_id}/transactions?page=1&page_size=10

Response:
[
  {
    "transaction_id": "550e8400-e29b-41d4-a716-446655440000",
    "type": "credit",
    "amount": 200,
    "timestamp": "2025-04-08T17:09:17Z"
  },
  ...
]
```

## Local Development

### Prerequisites
- Go 1.24 or later
- Docker and Docker Compose
- PostgreSQL (for local development)

### Quick Start

1. Clone the repository:
```bash
git clone <repository-url>
cd ledger-service
```

2. Set up environment variables:
```bash
cp .env.example .env
# Edit .env with your local settings
```

3. Start PostgreSQL using Docker:
```bash
docker-compose up -d db
```

4. Run database migrations:
```bash
docker exec -i ledger-service-db-1 psql -U ledger -d ledger_db < migrations/init.sql
```

5. Start the service:
```bash
go mod download
go run main.go
```

The service will be available at http://localhost:8080

### Docker Deployment

```bash
docker-compose up --build
```

## Testing

### Automated Tests
```bash
go test ./...
```

### Local Testing
Use curl or Postman to test the endpoints. Examples are provided in the API Endpoints section above.

## Implementation Details

### Concurrent Safety
- Uses database transactions with row-level locking
- Implements optimistic concurrency control
- Ensures atomic operations for balance updates

### Error Handling
- Validates input data
- Checks for insufficient balance
- Provides meaningful error messages
