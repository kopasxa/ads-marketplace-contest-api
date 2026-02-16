-- 002_user_wallets.up.sql
-- TON wallet connection with proof verification

CREATE TABLE user_wallets (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id         UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    address         TEXT NOT NULL,             -- raw TON address (0:<hex>)
    address_friendly TEXT NOT NULL,            -- user-friendly (EQ... / UQ...)
    network         TEXT NOT NULL DEFAULT 'mainnet', -- mainnet / testnet
    public_key      TEXT NOT NULL,             -- hex-encoded Ed25519 public key
    proof_payload   TEXT NOT NULL,             -- nonce/payload that was signed
    proof_signature TEXT NOT NULL,             -- hex-encoded signature
    proof_timestamp BIGINT NOT NULL,           -- unix timestamp from proof
    proof_domain    TEXT NOT NULL,             -- domain from proof (e.g. app.example.com)
    verified        BOOLEAN NOT NULL DEFAULT true,
    connected_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    disconnected_at TIMESTAMPTZ,
    is_active       BOOLEAN NOT NULL DEFAULT true,

    UNIQUE (user_id, address)
);

CREATE INDEX idx_user_wallets_user ON user_wallets(user_id) WHERE is_active = true;
CREATE INDEX idx_user_wallets_address ON user_wallets(address);

-- Таблица для payload/nonce выдачи (защита от replay)
CREATE TABLE ton_proof_payloads (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    payload     TEXT UNIQUE NOT NULL,
    user_id     UUID REFERENCES users(id),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at  TIMESTAMPTZ NOT NULL,
    used        BOOLEAN NOT NULL DEFAULT false
);

CREATE INDEX idx_ton_proof_payloads_payload ON ton_proof_payloads(payload) WHERE used = false;

-- Добавим wallet_address в users как кеш активного кошелька (опционально)
ALTER TABLE users ADD COLUMN wallet_address TEXT;

-- Привяжем withdraw_wallets к верифицированному кошельку
ALTER TABLE withdraw_wallets ADD COLUMN user_wallet_id UUID REFERENCES user_wallets(id);
