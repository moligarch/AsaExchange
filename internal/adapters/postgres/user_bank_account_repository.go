package postgres

import (
	"AsaExchange/internal/core/domain"
	"AsaExchange/internal/core/ports"
	"context"
	"encoding/base64"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/rs/zerolog"
)

var _ ports.UserBankAccountRepository = (*userBankAccountRepository)(nil)  // Ensure compliance

type userBankAccountRepository struct {
	db     *DB
	secSvc ports.SecurityPort
	log    zerolog.Logger
}

// NewUserBankAccountRepository creates a new repo for bank account operations.
func NewUserBankAccountRepository(db *DB, secSvc ports.SecurityPort, baseLogger *zerolog.Logger) ports.UserBankAccountRepository {
	return &userBankAccountRepository{
		db:     db,
		secSvc: secSvc,
		log:    baseLogger.With().Str("component", "user_bank_acct_repo").Logger(),
	}
}

// Create encrypts and saves a new bank account.
func (r *userBankAccountRepository) Create(ctx context.Context, acct *domain.UserBankAccount) error {
	// 1. Encrypt sensitive field
	encBytes, err := r.secSvc.Encrypt([]byte(acct.AccountDetails))
	if err != nil {
		r.log.Error().Err(err).Msg("Failed to encrypt account details")
		return err
	}
	// Encode to Base64
	encDetails := base64.StdEncoding.EncodeToString(encBytes)

	// 2. Insert into database
	query := `
		INSERT INTO user_bank_accounts (
			id, user_id, account_name, currency, bank_name, account_details
		) VALUES ($1, $2, $3, $4, $5, $6)
	`
	_, err = r.db.pool.Exec(ctx, query,
		acct.ID,
		acct.UserID,
		acct.AccountName,
		acct.Currency,
		acct.BankName,
		encDetails,
	)

	if err != nil {
		r.log.Error().Err(err).Str("user_id", acct.UserID.String()).Msg("Failed to insert new bank account")
	}
	return err
}

// scanAcct is a helper to scan a row and decrypt data.
func (r *userBankAccountRepository) scanAcct(row pgx.Row) (*domain.UserBankAccount, error) {
	var acct domain.UserBankAccount
	var encDetails string // Read encrypted data first

	err := row.Scan(
		&acct.ID,
		&acct.UserID,
		&acct.AccountName,
		&acct.Currency,
		&acct.BankName,
		&encDetails,
		&acct.CreatedAt,
		&acct.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, err
		}
		r.log.Error().Err(err).Msg("Failed to scan bank account row")
		return nil, err
	}

	// 2. Decrypt field
	decBytes, err := base64.StdEncoding.DecodeString(encDetails)
	if err != nil {
		r.log.Error().Err(err).Str("acct_id", acct.ID.String()).Msg("Failed to base64-decode account details")
		return nil, err
	}

	dec, err := r.secSvc.Decrypt(decBytes)
	if err != nil {
		r.log.Error().Err(err).Str("acct_id", acct.ID.String()).Msg("Failed to decrypt account details")
		return nil, err
	}

	acct.AccountDetails = string(dec)
	return &acct, nil
}

// GetByUserID finds all bank accounts for a given user.
func (r *userBankAccountRepository) GetByUserID(ctx context.Context, userID uuid.UUID) ([]*domain.UserBankAccount, error) {
	query := `
		SELECT id, user_id, account_name, currency, bank_name, 
			   account_details, created_at, updated_at
		FROM user_bank_accounts
		WHERE user_id = $1
		ORDER BY created_at DESC
	`

	rows, err := r.db.pool.Query(ctx, query, userID)
	if err != nil {
		r.log.Error().Err(err).Str("user_id", userID.String()).Msg("Failed to query bank accounts")
		return nil, err
	}
	defer rows.Close()

	var accounts []*domain.UserBankAccount
	for rows.Next() {
		acct, err := r.scanAcct(rows)
		if err != nil {
			// Log the error but try to continue processing other rows?
			// No, safer to fail the whole request.
			r.log.Error().Err(err).Str("user_id", userID.String()).Msg("Failed during row scan for bank accounts")
			return nil, err
		}
		accounts = append(accounts, acct)
	}

	if rows.Err() != nil {
		r.log.Error().Err(rows.Err()).Str("user_id", userID.String()).Msg("Error iterating bank account rows")
		return nil, rows.Err()
	}

	return accounts, nil
}
