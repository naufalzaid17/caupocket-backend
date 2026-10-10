package service

import "errors"

// Sentinel error business logic. Handler memetakan error ini ke HTTP status
// dan error_code pada Standard Response Wrapper.
var (
	ErrEmailTaken          = errors.New("email sudah terdaftar")
	ErrInvalidCredentials  = errors.New("email atau password salah")
	ErrInvalidInput        = errors.New("data yang dikirim tidak valid")
	ErrInvalidAmount       = errors.New("nominal harus lebih besar dari nol")
	ErrWalletNotFound      = errors.New("dompet tidak ditemukan")
	ErrGoalNotFound        = errors.New("target tabungan tidak ditemukan")
	ErrGoalAlreadyAchieved = errors.New("target tabungan sudah tercapai")
	ErrCategoryNotFound    = errors.New("kategori tidak ditemukan")
	ErrCategoryMismatch    = errors.New("tipe kategori tidak sesuai dengan tipe transaksi")
	ErrInsufficientBalance = errors.New("saldo dompet tidak cukup")
)
