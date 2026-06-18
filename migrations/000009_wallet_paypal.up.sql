-- 000009_wallet_paypal.up.sql
-- Withdrawals are paid out via PayPal Payouts (recipient email), not bank transfer.
-- Add the recipient PayPal email to withdrawal_requests; bank_* columns stay for
-- backward compatibility but are no longer used by the USD payout flow.

ALTER TABLE withdrawal_requests
    ADD COLUMN IF NOT EXISTS paypal_email TEXT NOT NULL DEFAULT '';
