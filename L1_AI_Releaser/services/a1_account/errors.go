// Package a1_account 提供 A1 模块的错误定义。
package a1_account

import "errors"

var (
	ErrUnauthorized    = errors.New("a1: unauthorized")
	ErrAccountNotFound = errors.New("a1: account not found")
	ErrInvalidInput    = errors.New("a1: invalid input")
	ErrKMSUnavailable  = errors.New("a1: KMS unavailable")
	ErrDecryptFailed   = errors.New("a1: decrypt failed")
)
