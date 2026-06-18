-- 000003_wallet.up.sql
-- Wallets, double-entry ledger, transactions, deposit addresses

CREATE TABLE IF NOT EXISTS wallets (
    wallet_id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id           UUID NOT NULL REFERENCES users(user_id) ON DELETE CASCADE,
    wallet_address    TEXT NOT NULL DEFAULT '',
    currency          TEXT NOT NULL,
    available_balance NUMERIC(36, 18) NOT NULL DEFAULT 0,
    locked_balance    NUMERIC(36, 18) NOT NULL DEFAULT 0,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (user_id, currency)
);

CREATE INDEX IF NOT EXISTS idx_wallets_user_id ON wallets(user_id);

CREATE TABLE IF NOT EXISTS ledger_entries (
    entry_id    UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    wallet_id   UUID NOT NULL REFERENCES wallets(wallet_id),
    type        TEXT NOT NULL CHECK (type IN ('debit', 'credit')),
    amount      NUMERIC(36, 18) NOT NULL,
    currency    TEXT NOT NULL,
    ref_type    TEXT NOT NULL,
    ref_id      UUID NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_ledger_entries_wallet_id ON ledger_entries(wallet_id);
CREATE INDEX IF NOT EXISTS idx_ledger_entries_ref       ON ledger_entries(ref_type, ref_id);

CREATE TABLE IF NOT EXISTS transactions (
    transaction_id TEXT PRIMARY KEY,
    user_id        UUID NOT NULL REFERENCES users(user_id),
    type           TEXT NOT NULL CHECK (type IN (
                       'deposit', 'withdrawal', 'trade_buy', 'trade_sell',
                       'fee', 'p2p_escrow', 'p2p_release')),
    amount         NUMERIC(36, 18) NOT NULL,
    currency       TEXT NOT NULL,
    status         TEXT NOT NULL DEFAULT 'pending'
                       CHECK (status IN ('pending', 'confirmed', 'failed')),
    tx_hash        TEXT,
    fee            NUMERIC(36, 18) NOT NULL DEFAULT 0,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_transactions_user_id ON transactions(user_id);

CREATE TABLE IF NOT EXISTS deposit_addresses (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id          UUID NOT NULL REFERENCES users(user_id) ON DELETE CASCADE,
    currency         TEXT NOT NULL,
    network          TEXT NOT NULL,
    address          TEXT NOT NULL UNIQUE,
    derivation_index INTEGER NOT NULL,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (user_id, currency, network)
);
