package main

import (
	"net/http"

	"drexa/internal/auth"
	"drexa/internal/payment"
)

func addRoutes(
	mux *http.ServeMux,
	authUc auth.AuthUsecase,
	kycUc auth.KycUsecase,
	adminKycUc auth.AdminKycUsecase,
	tokenSvc auth.TokenService,
	fbVerifier auth.FirebaseVerifier,
	paymentUc payment.PaymentUsecase,
) {
	mux.Handle("/", http.NotFoundHandler())

	jwt := auth.JWTMiddleware(tokenSvc)

	// ── Public auth ──────────────────────────────────────────────────────────
	mux.Handle("POST /api/v1/auth/signin", auth.HandleFirebaseSignIn(authUc, fbVerifier))
	mux.Handle("POST /api/v1/auth/logout", auth.HandleLogout(authUc))
	mux.Handle("POST /api/v1/auth/refresh", auth.HandleRefreshToken(authUc))

	// ── Protected auth (JWT required) ────────────────────────────────────────
	mux.Handle("POST /api/v1/auth/logout/all", jwt(auth.HandleLogoutAll(authUc)))
	mux.Handle("POST /api/v1/auth/pin/set", jwt(auth.HandleSetTradingPin(authUc)))
	mux.Handle("POST /api/v1/auth/pin/verify", jwt(auth.HandleVerifyTradingPin(authUc)))
	mux.Handle("POST /api/v1/auth/verify/phone", jwt(auth.HandleVerifyPhone(authUc)))

	// ── KYC — user facing (JWT required) ─────────────────────────────────────
	_ = kycUc // TODO: implement KYC handlers

	// ── KYC — admin facing (JWT required) ────────────────────────────────────
	_ = adminKycUc // TODO: implement admin KYC handlers

	// ── Payments — public webhook (verified via Stripe signature) ────────────
	mux.Handle("POST /api/v1/payments/webhook", payment.HandleStripeWebhook(paymentUc))

	// ── Payments — protected (JWT required) ──────────────────────────────────
	mux.Handle("POST /api/v1/payments/deposit/intent", jwt(payment.HandleCreateDepositIntent(paymentUc)))
	mux.Handle("POST /api/v1/payments/withdraw", jwt(payment.HandleWithdraw(paymentUc)))

	// ── Wallet — protected (JWT required) ────────────────────────────────────
	mux.Handle("GET /api/v1/wallet/balance", jwt(payment.HandleGetBalance(paymentUc)))
	mux.Handle("GET /api/v1/wallet/transactions", jwt(payment.HandleGetTransactions(paymentUc)))
}
