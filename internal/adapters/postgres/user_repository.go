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

type userRepository struct {
	db     *DB
	secSvc ports.SecurityPort // We need this to encrypt/decrypt
	log    zerolog.Logger
}

var _ ports.UserRepository = (*userRepository)(nil) // Ensure compliance

// NewUserRepository creates a new repository for user operations.
func NewUserRepository(db *DB, secSvc ports.SecurityPort, baseLogger *zerolog.Logger) ports.UserRepository {
	return &userRepository{
		db:     db,
		secSvc: secSvc,
		log:    baseLogger.With().Str("component", "user_repo").Logger(),
	}
}

// Create encrypts sensitive data and saves a new user.
func (r *userRepository) Create(ctx context.Context, user *domain.User) error {
	// 1. Encrypt sensitive fields
	var err error
	var encPhone, encGovID *string

	if user.PhoneNumber != nil {
		encBytes, err := r.secSvc.Encrypt([]byte(*user.PhoneNumber))
		if err != nil {
			r.log.Error().Err(err).Msg("Failed to encrypt phone number")
			return err
		}
		encStr := base64.StdEncoding.EncodeToString(encBytes)
		encPhone = &encStr
	}

	if user.GovernmentID != nil {
		encBytes, err := r.secSvc.Encrypt([]byte(*user.GovernmentID))
		if err != nil {
			r.log.Error().Err(err).Msg("Failed to encrypt government ID")
			return err
		}
		encStr := base64.StdEncoding.EncodeToString(encBytes)
		encGovID = &encStr
	}

	// 2. Insert into database
	query := `
		INSERT INTO users (
			id, telegram_id, first_name, last_name, phone_number,
			government_id, location_country, verification_status, user_state, 
			verification_strategy, identity_doc_ref, is_moderator
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
	`
	_, err = r.db.pool.Exec(ctx, query,
		user.ID,
		user.TelegramID,
		user.FirstName,
		user.LastName,
		encPhone,
		encGovID,
		user.LocationCountry,
		user.VerificationStatus,
		user.State,
		user.VerificationStrategy,
		user.IdentityDocRef,
		user.IsModerator,
	)

	if err != nil {
		r.log.Error().Err(err).Int64("telegram_id", user.TelegramID).Msg("Failed to insert new user")
	}
	return err
}

// scanUser is a helper to scan a row into a User struct
// It handles decryption internally.
func (r *userRepository) scanUser(row pgx.Row) (*domain.User, error) {
	var user domain.User
	var encPhone, encGovID *string // Read encrypted data first

	err := row.Scan(
		&user.ID,
		&user.TelegramID,
		&user.FirstName,
		&user.LastName,
		&encPhone,
		&encGovID,
		&user.LocationCountry,
		&user.VerificationStatus,
		&user.State,
		&user.VerificationStrategy,
		&user.IdentityDocRef,
		&user.IsModerator,
		&user.CreatedAt,
		&user.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, err // Return specific error
		}
		r.log.Error().Err(err).Msg("Failed to scan user row")
		return nil, err
	}

	// 2. Decrypt fields
	if encPhone != nil {
		// 1. Decode from Base64 string to raw bytes
		decBytes, err := base64.StdEncoding.DecodeString(*encPhone)
		if err != nil {
			r.log.Error().Err(err).Str("user_id", user.ID.String()).Msg("Failed to base64-decode phone number")
			return nil, err // Fail the request
		}

		// 2. Decrypt the raw bytes
		dec, err := r.secSvc.Decrypt(decBytes)
		if err != nil {
			r.log.Error().Err(err).Str("user_id", user.ID.String()).Msg("Failed to decrypt phone number (tampered?)")
			return nil, err // Fail the request
		}
		decStr := string(dec)
		user.PhoneNumber = &decStr
	}

	if encGovID != nil {
		decBytes, err := base64.StdEncoding.DecodeString(*encGovID)
		if err != nil {
			r.log.Error().Err(err).Str("user_id", user.ID.String()).Msg("Failed to base64-decode gov ID")
			return nil, err // Fail the request
		}

		dec, err := r.secSvc.Decrypt(decBytes)
		if err != nil {
			r.log.Error().Err(err).Str("user_id", user.ID.String()).Msg("Failed to decrypt gov ID (tampered?)")
			return nil, err // Fail the request
		}
		decStr := string(dec)
		user.GovernmentID = &decStr
	}

	return &user, nil
}

// sharedQuery is the list of columns for scanning
const userQueryCols = `
	id, telegram_id, first_name, last_name, phone_number,
	government_id, location_country, verification_status, user_state, 
	verification_strategy, identity_doc_ref, is_moderator,
	created_at, updated_at
`

// GetByTelegramID finds and decrypts a user by their Telegram ID.
func (r *userRepository) GetByTelegramID(ctx context.Context, telegramID int64) (*domain.User, error) {
	query := `SELECT ` + userQueryCols + ` FROM users WHERE telegram_id = $1`

	row := r.db.pool.QueryRow(ctx, query, telegramID)
	user, err := r.scanUser(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			r.log.Info().Int64("telegram_id", telegramID).Msg("User not found")
			return nil, nil // Return nil, nil for "not found"
		}
		return nil, err
	}
	return user, nil
}

// GetByID finds and decrypts a user by their internal UUID.
func (r *userRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.User, error) {
	query := `SELECT ` + userQueryCols + ` FROM users WHERE id = $1`

	row := r.db.pool.QueryRow(ctx, query, id)
	user, err := r.scanUser(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			r.log.Info().Str("user_id", id.String()).Msg("User not found")
			return nil, nil // Return nil, nil for "not found"
		}
		return nil, err
	}
	return user, nil
}

// Update saves all fields of the user struct, re-encrypting sensitive data.
func (r *userRepository) Update(ctx context.Context, user *domain.User) error {
	// 1. Re-encrypt sensitive fields
	var err error
	var encPhone, encGovID *string

	if user.PhoneNumber != nil {
		encBytes, err := r.secSvc.Encrypt([]byte(*user.PhoneNumber))
		if err != nil {
			r.log.Error().Err(err).Msg("Failed to encrypt phone number for update")
			return err
		}
		encStr := base64.StdEncoding.EncodeToString(encBytes)
		encPhone = &encStr
	}
	if user.GovernmentID != nil {
		encBytes, err := r.secSvc.Encrypt([]byte(*user.GovernmentID))
		if err != nil {
			r.log.Error().Err(err).Msg("Failed to encrypt government ID for update")
			return err
		}
		encStr := base64.StdEncoding.EncodeToString(encBytes)
		encGovID = &encStr
	}

	// 2. Run the update query
	query := `
		UPDATE users SET
			first_name = $1,
			last_name = $2,
			phone_number = $3,
			government_id = $4,
			location_country = $5,
			verification_status = $6,
			user_state = $7,
			is_moderator = $8,
			verification_strategy = $9,
			identity_doc_ref = $10,
			updated_at = NOW()
		WHERE id = $11
	`
	cmdTag, err := r.db.pool.Exec(ctx, query,
		user.FirstName,
		user.LastName,
		encPhone,
		encGovID,
		user.LocationCountry,
		user.VerificationStatus,
		user.State,
		user.IsModerator,
		user.VerificationStrategy,
		user.IdentityDocRef,
		user.ID, // The WHERE clause
	)

	if err != nil {
		r.log.Error().Err(err).Str("user_id", user.ID.String()).Msg("Failed to update user")
		return err
	}

	if cmdTag.RowsAffected() == 0 {
		r.log.Error().Err(errors.New("no rows affected")).Str("user_id", user.ID.String()).Msg("User not found when trying to update")
		return errors.New("user not found")
	}

	return nil
}

// Delete removes a user from the database.
func (r *userRepository) Delete(ctx context.Context, id uuid.UUID) error {
	query := `DELETE FROM users WHERE id = $1`

	cmdTag, err := r.db.pool.Exec(ctx, query, id)
	if err != nil {
		r.log.Error().Err(err).Str("user_id", id.String()).Msg("Failed to delete user")
		return err
	}

	if cmdTag.RowsAffected() == 0 {
		r.log.Error().Err(errors.New("no rows affected")).Str("user_id", id.String()).Msg("User not found when trying to delete")
		return errors.New("user not found")
	}

	return nil
}

// GetNextPendingUser finds the oldest user in 'pending' status
func (r *userRepository) GetNextPendingUser(ctx context.Context) (*domain.User, error) {
	query := `SELECT ` + userQueryCols + ` FROM users 
		WHERE verification_status = $1
		ORDER BY created_at ASC
		LIMIT 1
	`

	row := r.db.pool.QueryRow(ctx, query, domain.VerificationPending)
	user, err := r.scanUser(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil // No pending users, not an error
		}
		return nil, err
	}
	return user, nil
}
