-- 000011_seed_market.up.sql
-- Seed initial coins and trading pairs for testing

INSERT INTO coins (coin_id, symbol, name, decimals, network, status)
VALUES 
    ('BTC', 'BTC', 'Bitcoin', 8, 'Bitcoin', 'active'),
    ('USD', 'USD', 'US Dollar', 2, 'Fiat', 'active'),
    ('ETH', 'ETH', 'Ethereum', 18, 'Ethereum', 'active')
ON CONFLICT (coin_id) DO NOTHING;

INSERT INTO trading_pairs (pair_id, base_coin, quote_coin, status, min_order_size, price_decimal_places)
VALUES 
    ('BTC_USD', 'BTC', 'USD', 'active', 0.0001, 2),
    ('ETH_USD', 'ETH', 'USD', 'active', 0.01, 2)
ON CONFLICT (pair_id) DO NOTHING;
