-- This file defines our ENUM types and all tables.

-- --- 1. DEFINE CUSTOM TYPES (ENUMS) ---

CREATE TYPE transaction_status AS ENUM (
    'pending_deposits',       -- Initial state
    'seller_deposit_received',-- Seller (EUR) sent money to us
    'buyer_deposit_received', -- Buyer (Rial) sent money to us
    'pending_payouts',      -- We received both, ready to pay out
    'seller_payout_sent',     -- We sent Rial to the seller
    'buyer_payout_sent',      -- We sent EUR to the buyer
    'completed',              -- Both payouts sent
    'disputed',               -- An issue was raised
    'cancelled'               -- Transaction was cancelled
);

CREATE TYPE user_verification_status AS ENUM (
    'pending',  -- New user
    'level_1',  -- ID verified
    'rejected'  -- ID was invalid
);

CREATE TYPE request_type AS ENUM (
    'sell',
    'buy'
);

CREATE TYPE request_status AS ENUM (
    'open',
    'matched',  -- A bid was accepted
    'completed',
    'cancelled'
);

CREATE TYPE bid_status AS ENUM (
    'pending',
    'accepted',
    'rejected',
    'cancelled'
);


-- --- 2. CREATE TABLES ---

-- Users Table
CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    telegram_id BIGINT NOT NULL UNIQUE,
    first_name TEXT NOT NULL,
    last_name TEXT,
    phone_number TEXT,              -- Encrypted
    government_id TEXT,             -- Encrypted
    location_country VARCHAR(3),    -- ISO 3166-1 alpha-3
    verification_status user_verification_status NOT NULL DEFAULT 'pending',
    is_moderator BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Platform Accounts (Our company's bank accounts)
CREATE TABLE platform_accounts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    account_name TEXT NOT NULL,         -- e.g., "Primary Rial - Mellat"
    currency VARCHAR(3) NOT NULL,       -- e.g., 'EUR', 'Rial'
    bank_name TEXT NOT NULL,            -- e.g., "Bank Mellat", "N26"
    account_details TEXT NOT NULL,      -- User-facing info (IBAN, card number)
    verification_strategy TEXT NOT NULL DEFAULT 'manual', -- 'manual', 'bank_api_mellat'
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- User Bank Accounts (User's payout accounts)
CREATE TABLE user_bank_accounts (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    account_name TEXT NOT NULL,         -- e.g., "My Mellat Account"
    currency VARCHAR(3) NOT NULL,
    bank_name TEXT NOT NULL,
    account_details TEXT NOT NULL,      -- Encrypted (IBAN, card number)
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Requests Table (The marketplace "ads")
CREATE TABLE requests (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    channel_message_id BIGINT,          -- Telegram message ID in the channel
    request_type request_type NOT NULL, -- 'sell' or 'buy'
    base_currency VARCHAR(3) NOT NULL,  -- e.g., 'EUR'
    quote_currency VARCHAR(3) NOT NULL, -- e.g., 'Rial'
    base_amount NUMERIC(19, 8) NOT NULL,
    exchange_rate NUMERIC(19, 8) NOT NULL,
    status request_status NOT NULL DEFAULT 'open',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Bids Table (User bids on requests)
CREATE TABLE bids (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    request_id UUID NOT NULL REFERENCES requests(id) ON DELETE CASCADE,
    status bid_status NOT NULL DEFAULT 'pending',
    notes TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(user_id, request_id) -- A user can only bid once per request
);

-- Transactions Table (The fulfillment ledger)
CREATE TABLE transactions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    request_id UUID NOT NULL REFERENCES requests(id),
    bid_id UUID NOT NULL REFERENCES bids(id),
    seller_user_id UUID NOT NULL REFERENCES users(id),
    buyer_user_id UUID NOT NULL REFERENCES users(id),
    moderator_id UUID REFERENCES users(id), -- Mod who handled it
    
    status transaction_status NOT NULL DEFAULT 'pending_deposits',

    -- Deposit Leg (Users paying IN to us)
    platform_deposit_base_account_id UUID REFERENCES platform_accounts(id), -- Platform's EUR account
    platform_deposit_quote_account_id UUID REFERENCES platform_accounts(id), -- Platform's Rial account

    -- Payout Leg (Us paying OUT to users)
    seller_payout_account_id UUID REFERENCES user_bank_accounts(id), -- User's Rial account
    buyer_payout_account_id UUID REFERENCES user_bank_accounts(id),  -- User's EUR account

    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);


-- --- 3. CREATE INDEXES ---

CREATE INDEX ON users (telegram_id);
CREATE INDEX ON user_bank_accounts (user_id);
CREATE INDEX ON platform_accounts (currency);
CREATE INDEX ON requests (user_id);
CREATE INDEX ON requests (status, base_currency, quote_currency);
CREATE INDEX ON bids (user_id);
CREATE INDEX ON bids (request_id);
CREATE INDEX ON transactions (status);
CREATE INDEX ON transactions (seller_user_id);
CREATE INDEX ON transactions (buyer_user_id);
CREATE INDEX ON transactions (moderator_id);