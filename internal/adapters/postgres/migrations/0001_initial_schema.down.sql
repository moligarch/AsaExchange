-- Drop in reverse order of creation
DROP TABLE IF EXISTS transactions;
DROP TABLE IF EXISTS bids;
DROP TABLE IF EXISTS requests;
DROP TABLE IF EXISTS user_bank_accounts;
DROP TABLE IF EXISTS platform_accounts;
DROP TABLE IF EXISTS users;

-- Drop types
DROP TYPE IF EXISTS transaction_status;
DROP TYPE IF EXISTS user_verification_status;
DROP TYPE IF EXISTS request_type;
DROP TYPE IF EXISTS request_status;
DROP TYPE IF EXISTS bid_status;
DROP TYPE IF EXISTS user_state_enum;