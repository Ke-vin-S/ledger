# Auth Reference

## Token model

| Token | Type | TTL | Storage |
|---|---|---|---|
| Access token | JWT RS256 | 15 min | JS memory (backend: stateless) |
| Refresh token | Opaque random string | 30 days | Hashed in Redis + HttpOnly cookie |

## JWT claims

```json
{
  "sub": "user-uuid",
  "iat": 1234567890,
  "exp": 1234568790,
  "jti": "unique-token-id",
  "identity_type": "registered"
}
```

## Refresh token rotation

1. Client sends `POST /auth/refresh` with HttpOnly cookie (web) or body (mobile)
2. Server looks up hash in Redis — if not found → 401
3. Server deletes the old token from Redis (revocation)
4. Server issues new access token + new refresh token
5. New refresh token stored hashed in Redis, returned in HttpOnly cookie (web) / body (mobile)

**Family invalidation**: if a refresh token is used that was already revoked (replay attack), invalidate ALL refresh tokens for that user in Redis. Force re-login.

## Redis key structure

```
refresh:<token_hash>   → user_id (TTL: 30 days)
revoked:<jti>          → "1"    (TTL: 15 min — same as access token TTL)
```

## RS256 key management

- Private key: SSM `/splitleger/jwt_private_key` (PEM). Loaded at startup. Used to sign tokens.
- Public key: SSM `/splitleger/jwt_public_key` (PEM). Used to verify. Can also be served at `GET /.well-known/jwks.json`.
- Generate locally: `openssl genrsa -out private.pem 2048 && openssl rsa -in private.pem -pubout -out public.pem`

## Anonymous users and auth

Anonymous users (`identity_type = 'anonymous'`) have no login credentials and cannot obtain tokens. They are created by registered users and participate in splits as data entities only.

Claim flow: registered user authenticates normally → presents claim token → server merges anon profile → responds with updated user profile.

## CORS

API must allow:
- `https://splitleger.app` (production)
- `http://localhost:3000` (local dev)

Credentials mode (`credentials: "include"`) required for the HttpOnly refresh cookie to be sent cross-origin.
