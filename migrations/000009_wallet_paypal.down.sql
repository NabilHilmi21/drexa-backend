-- 000009_wallet_paypal.down.sql

ALTER TABLE withdrawal_requests
    DROP COLUMN IF EXISTS paypal_email;
