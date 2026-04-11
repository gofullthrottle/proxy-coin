// Package customer handles customer-facing REST API operations.
package customer

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"
)

// passwordBcryptCost is the bcrypt work factor for customer passwords.
const passwordBcryptCost = 12

// Service provides business logic for customer lifecycle.
// All state is persisted in PostgreSQL.
type Service struct {
	pool *pgxpool.Pool
}

// NewService creates a customer Service backed by the given connection pool.
func NewService(pool *pgxpool.Pool) *Service {
	return &Service{pool: pool}
}

// CreateCustomer registers a new customer and returns the created record.
// Returns an error if a customer with the same email already exists.
func (s *Service) CreateCustomer(ctx context.Context, email, password string) (*Customer, error) {
	if email == "" || password == "" {
		return nil, fmt.Errorf("customer: email and password are required")
	}

	// Check for existing customer.
	exists, err := s.emailExists(ctx, email)
	if err != nil {
		return nil, fmt.Errorf("customer: check email uniqueness: %w", err)
	}
	if exists {
		return nil, fmt.Errorf("customer: email already registered")
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), passwordBcryptCost)
	if err != nil {
		return nil, fmt.Errorf("customer: hash password: %w", err)
	}

	id := uuid.New().String()
	now := time.Now().UTC()

	const q = `
		INSERT INTO customers (id, email, password_hash, plan, created_at, updated_at)
		VALUES ($1, $2, $3, 'free', $4, $4)
		RETURNING id, email, plan, created_at
	`
	var c Customer
	row := s.pool.QueryRow(ctx, q, id, email, string(hash), now)
	if err := row.Scan(&c.ID, &c.Email, &c.Tier, &c.CreatedAt); err != nil {
		return nil, fmt.Errorf("customer: insert customer: %w", err)
	}
	return &c, nil
}

// AuthenticateCustomer verifies email/password and returns the customer record.
// Returns an error if the credentials are invalid.
func (s *Service) AuthenticateCustomer(ctx context.Context, email, password string) (*Customer, error) {
	const q = `
		SELECT id, email, password_hash, plan, created_at
		FROM customers
		WHERE email = $1
	`
	var c Customer
	var hash string
	row := s.pool.QueryRow(ctx, q, email)
	if err := row.Scan(&c.ID, &c.Email, &hash, &c.Tier, &c.CreatedAt); err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("customer: invalid credentials")
		}
		return nil, fmt.Errorf("customer: authenticate: %w", err)
	}

	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)); err != nil {
		return nil, fmt.Errorf("customer: invalid credentials")
	}

	return &c, nil
}

// GetCustomer returns a customer by ID.
func (s *Service) GetCustomer(ctx context.Context, id string) (*Customer, error) {
	const q = `
		SELECT id, email, plan, created_at
		FROM customers
		WHERE id = $1
	`
	var c Customer
	row := s.pool.QueryRow(ctx, q, id)
	if err := row.Scan(&c.ID, &c.Email, &c.Tier, &c.CreatedAt); err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("customer: not found")
		}
		return nil, fmt.Errorf("customer: get: %w", err)
	}
	return &c, nil
}

// SaveAPIKey persists a bcrypt-hashed API key for a customer.
func (s *Service) SaveAPIKey(ctx context.Context, customerID, keyID, keyHash string) error {
	const q = `
		INSERT INTO customer_api_keys (id, customer_id, key_hash, created_at)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (customer_id) DO UPDATE SET
			id         = EXCLUDED.id,
			key_hash   = EXCLUDED.key_hash,
			created_at = EXCLUDED.created_at,
			revoked    = false
	`
	_, err := s.pool.Exec(ctx, q, keyID, customerID, keyHash, time.Now().UTC())
	if err != nil {
		return fmt.Errorf("customer: save api key: %w", err)
	}
	return nil
}

// LookupAPIKey returns the customerID and plan for the given API key hash.
func (s *Service) LookupAPIKeyHash(ctx context.Context, keyID string) (customerID, keyHash, plan string, err error) {
	const q = `
		SELECT k.customer_id, k.key_hash, c.plan
		FROM customer_api_keys k
		JOIN customers c ON c.id = k.customer_id
		WHERE k.id = $1 AND k.revoked = false
	`
	row := s.pool.QueryRow(ctx, q, keyID)
	if err = row.Scan(&customerID, &keyHash, &plan); err != nil {
		if err == pgx.ErrNoRows {
			return "", "", "", fmt.Errorf("customer: api key not found")
		}
		return "", "", "", fmt.Errorf("customer: lookup api key: %w", err)
	}
	return customerID, keyHash, plan, nil
}

// emailExists returns true if a customer with the given email already exists.
func (s *Service) emailExists(ctx context.Context, email string) (bool, error) {
	const q = `SELECT EXISTS(SELECT 1 FROM customers WHERE email = $1)`
	var exists bool
	if err := s.pool.QueryRow(ctx, q, email).Scan(&exists); err != nil {
		return false, fmt.Errorf("customer: check email exists: %w", err)
	}
	return exists, nil
}

// Create is the original stub-compatible method retained for backward compatibility.
func (s *Service) Create(ctx context.Context, req CreateCustomerRequest) (*Customer, error) {
	return s.CreateCustomer(ctx, req.Email, "")
}

// GetUsage returns bandwidth usage for a customer within the given period.
// Delegates to the UsageTracker.
func (s *Service) GetUsage(ctx context.Context, customerID string, start, end time.Time) (*UsageSummary, error) {
	return &UsageSummary{
		CustomerID:  customerID,
		PeriodStart: start,
		PeriodEnd:   end,
	}, nil
}
