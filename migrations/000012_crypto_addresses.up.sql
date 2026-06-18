-- 000012_crypto_addresses.up.sql
-- Per-user on-chain deposit addresses derived from the master HD wallet (Tatum).
-- One address per (user, currency); identified on-chain by `address`, credited to the
-- internal ledger when a deposit webhook / balance check confirms funds.

CREATE TABLE IF NOT EXISTS crypto_addresses (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id          UUID NOT NULL REFERENCES users(user_id) ON DELETE CASCADE,
    currency         TEXT NOT NULL,
    chain            TEXT NOT NULL,
    address          TEXT NOT NULL,
    xpub             TEXT NOT NULL DEFAULT '',
    derivation_index INTEGER NOT NULL DEFAULT 0,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (user_id, currency)
);

CREATE INDEX IF NOT EXISTS idx_crypto_addresses_address ON crypto_addresses(address);
CREATE INDEX IF NOT EXISTS idx_crypto_addresses_chain   ON crypto_addresses(chain);
