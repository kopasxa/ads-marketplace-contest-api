-- 002_user_wallets.down.sql
ALTER TABLE withdraw_wallets DROP COLUMN IF EXISTS user_wallet_id;
ALTER TABLE users DROP COLUMN IF EXISTS wallet_address;
DROP TABLE IF EXISTS ton_proof_payloads;
DROP TABLE IF EXISTS user_wallets;
