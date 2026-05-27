package auth

import "errors"

var (
	ErrRefreshTokenInvalid = errors.New("refresh token is invalid or expired")
	ErrTokenRevoked        = errors.New("token has been revoked")
)
