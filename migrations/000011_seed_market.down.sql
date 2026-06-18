-- 000011_seed_market.down.sql
-- Remove the seeded USDC pairs and their coins. Pairs first (FK to coins).

DELETE FROM trading_pairs WHERE pair_id IN (
    'BTC_USDC','ETH_USDC','SOL_USDC','BNB_USDC','XRP_USDC','ADA_USDC','AVAX_USDC',
    'LINK_USDC','DOGE_USDC','DOT_USDC','MATIC_USDC','LTC_USDC','UNI_USDC',
    'ATOM_USDC','XLM_USDC','NEAR_USDC','APT_USDC'
);

DELETE FROM coins WHERE coin_id IN (
    'USDC','BTC','ETH','SOL','BNB','XRP','ADA','AVAX','LINK','DOGE','DOT',
    'MATIC','LTC','UNI','ATOM','XLM','NEAR','APT'
);

DELETE FROM trading_pairs WHERE pair_id IN ('BTC_USD', 'ETH_USD');
DELETE FROM coins WHERE coin_id IN ('BTC', 'USD', 'ETH');
