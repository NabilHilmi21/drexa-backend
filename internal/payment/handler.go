package payment

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"

	"drexa/internal/auth"
)

// ─── DTOs ────────────────────────────────────────────────────────────────────

type DepositIntentRequest struct {
	Amount int64 `json:"amount"` // in cents, e.g. 1000 = $10.00
}

type DepositIntentResponse struct {
	ClientSecret string `json:"client_secret"`
	TxID         string `json:"tx_id"`
}

type WithdrawRequest struct {
	Amount int64 `json:"amount"` // in cents
}

type BalanceResponse struct {
	Balance  int64  `json:"balance"`  // in cents
	Currency string `json:"currency"`
}

type MessageResponse struct {
	Message string `json:"message,omitempty"`
	Error   string `json:"error,omitempty"`
}

// ─── Helpers ─────────────────────────────────────────────────────────────────

func sendJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(payload)
}

func userFromRequest(r *http.Request) (string, bool) {
	claims, ok := auth.UserFromContext(r.Context())
	if !ok {
		return "", false
	}
	return claims.UserID, true
}

func domainErrToStatus(err error) int {
	switch {
	case errors.Is(err, ErrInvalidAmount),
		errors.Is(err, ErrMinimumDeposit),
		errors.Is(err, ErrMinimumWithdrawal):
		return http.StatusBadRequest
	case errors.Is(err, ErrInsufficientFunds):
		return http.StatusUnprocessableEntity
	case errors.Is(err, ErrWalletNotFound),
		errors.Is(err, ErrTransactionNotFound):
		return http.StatusNotFound
	default:
		return http.StatusInternalServerError
	}
}

// ─── Handlers ────────────────────────────────────────────────────────────────

// HandleCreateDepositIntent creates a Stripe PaymentIntent.
// The frontend uses the returned client_secret to confirm the payment via Stripe.js.
func HandleCreateDepositIntent(uc PaymentUsecase) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, ok := userFromRequest(r)
		if !ok {
			sendJSON(w, http.StatusUnauthorized, MessageResponse{Error: "unauthorized"})
			return
		}

		var req DepositIntentRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			sendJSON(w, http.StatusBadRequest, MessageResponse{Error: "invalid input"})
			return
		}

		clientSecret, txID, err := uc.CreateDepositIntent(r.Context(), userID, req.Amount)
		if err != nil {
			sendJSON(w, domainErrToStatus(err), MessageResponse{Error: err.Error()})
			return
		}

		sendJSON(w, http.StatusCreated, DepositIntentResponse{
			ClientSecret: clientSecret,
			TxID:         txID,
		})
	}
}

// HandleStripeWebhook receives Stripe events. It must NOT be behind JWTMiddleware
// because Stripe calls it directly. Authenticity is verified via webhook signature.
func HandleStripeWebhook(uc PaymentUsecase) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Stripe recommends reading at most 65536 bytes.
		payload, err := io.ReadAll(io.LimitReader(r.Body, 65536))
		if err != nil {
			sendJSON(w, http.StatusBadRequest, MessageResponse{Error: "cannot read body"})
			return
		}

		sig := r.Header.Get("Stripe-Signature")
		if err := uc.HandleWebhook(r.Context(), payload, sig); err != nil {
			sendJSON(w, http.StatusBadRequest, MessageResponse{Error: err.Error()})
			return
		}

		w.WriteHeader(http.StatusOK)
	}
}

// HandleWithdraw debits the user's wallet balance.
func HandleWithdraw(uc PaymentUsecase) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, ok := userFromRequest(r)
		if !ok {
			sendJSON(w, http.StatusUnauthorized, MessageResponse{Error: "unauthorized"})
			return
		}

		var req WithdrawRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			sendJSON(w, http.StatusBadRequest, MessageResponse{Error: "invalid input"})
			return
		}

		if err := uc.Withdraw(r.Context(), userID, req.Amount); err != nil {
			sendJSON(w, domainErrToStatus(err), MessageResponse{Error: err.Error()})
			return
		}

		sendJSON(w, http.StatusOK, MessageResponse{Message: "withdrawal recorded"})
	}
}

// HandleGetBalance returns the user's current wallet balance.
func HandleGetBalance(uc PaymentUsecase) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, ok := userFromRequest(r)
		if !ok {
			sendJSON(w, http.StatusUnauthorized, MessageResponse{Error: "unauthorized"})
			return
		}

		wallet, err := uc.GetBalance(r.Context(), userID)
		if err != nil {
			sendJSON(w, http.StatusInternalServerError, MessageResponse{Error: err.Error()})
			return
		}

		sendJSON(w, http.StatusOK, BalanceResponse{
			Balance:  wallet.Balance,
			Currency: wallet.Currency,
		})
	}
}

// HandleGetTransactions returns the user's paginated transaction history.
func HandleGetTransactions(uc PaymentUsecase) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, ok := userFromRequest(r)
		if !ok {
			sendJSON(w, http.StatusUnauthorized, MessageResponse{Error: "unauthorized"})
			return
		}

		limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
		offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))

		txs, err := uc.GetTransactions(r.Context(), userID, limit, offset)
		if err != nil {
			sendJSON(w, http.StatusInternalServerError, MessageResponse{Error: err.Error()})
			return
		}

		sendJSON(w, http.StatusOK, txs)
	}
}
