package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"time"

	"github.com/rs/zerolog/log"
	"gorm.io/gorm"

	"drexa/internal/auth"
	authRepo "drexa/internal/auth/repository"
	authSvc "drexa/internal/auth/service"
	authUc "drexa/internal/auth/usecase"
	"drexa/internal/checkout"
	"drexa/internal/kyc"
	kycRepo "drexa/internal/kyc/repository"
	kycSvc "drexa/internal/kyc/service"
	kycUc "drexa/internal/kyc/usecase"
	"drexa/internal/market"
	"drexa/internal/matching"
	"drexa/internal/order"
	orderRepo "drexa/internal/order/repository"
	"drexa/internal/platform/middleware"
	walletRepo "drexa/internal/wallet/repository"
	walletSvc "drexa/internal/wallet/service"
	walletUc "drexa/internal/wallet/usecase"
	"drexa/pkg/config"
)

// kycUserServiceAdapter bridges auth.UserRepository to kyc.UserService
// so the kyc domain never imports internal/auth.
type kycUserServiceAdapter struct {
	repo auth.UserRepository
}

func (a *kycUserServiceAdapter) FindByID(ctx context.Context, userID string) (*kyc.UserSnapshot, error) {
	u, err := a.repo.FindByID(ctx, userID)
	if err != nil {
		return nil, kyc.ErrUserNotFound
	}
	return &kyc.UserSnapshot{UserID: u.UserID, Email: u.Email}, nil
}

func (a *kycUserServiceAdapter) UpdateKycLevel(ctx context.Context, userID, reviewedBy string, level int) error {
	return a.repo.UpdateKycLevel(ctx, userID, reviewedBy, level)
}

// depthSourceAdapter bridges the order service to market.DepthSource so the
// market feed reads order-book depth without importing internal/order.
type depthSourceAdapter struct {
	orders order.Service
}

func (a *depthSourceAdapter) OrderBookDepth(ctx context.Context, pairID string, maxLevels int) (*market.OrderBookSnapshot, error) {
	snap, err := a.orders.OrderBookDepth(ctx, pairID, maxLevels)
	if err != nil {
		return nil, err
	}
	return &market.OrderBookSnapshot{
		PairID:  snap.PairID,
		Version: snap.Version,
		Bids:    toMarketLevels(snap.Bids),
		Asks:    toMarketLevels(snap.Asks),
	}, nil
}

func toMarketLevels(levels []order.OrderBookLevel) []market.BookLevel {
	out := make([]market.BookLevel, len(levels))
	for i, l := range levels {
		out[i] = market.BookLevel{Price: l.Price, Quantity: l.Quantity}
	}
	return out
}

// pairListerAdapter lists active trading pairs from the market.TradingPair
// table, satisfying market.PairLister.
type pairListerAdapter struct {
	db *gorm.DB
}

func (a *pairListerAdapter) ActivePairIDs(ctx context.Context) ([]string, error) {
	var ids []string
	err := a.db.WithContext(ctx).
		Model(&market.TradingPair{}).
		Where("status = ?", market.StatusActive).
		Pluck("pair_id", &ids).Error
	return ids, err
}

type Server struct {
	httpServer *http.Server
}

func NewServer(cfg *config.Config, db *gorm.DB) *Server {
	// ── Auth repositories ─────────────────────────────────────────────────────
	userRepo := authRepo.NewUserRepository(db)
	refreshTokenRepo := authRepo.NewRefreshTokenRepository(db)
	otpRepo := authRepo.NewOTPRepository(db)

	// ── Auth services ─────────────────────────────────────────────────────────
	emailSender := authSvc.NewSendGridEmailSender(cfg.SendGrid.APIKey, cfg.SendGrid.FromEmail, cfg.SendGrid.FromName)
	smsSender := authSvc.NewTwilioSMSSender(cfg.Twilio.AccountSID, cfg.Twilio.AuthToken, cfg.Twilio.FromPhone)
	otpService := authSvc.NewOTPService(otpRepo, emailSender, smsSender)
	tokenService := authSvc.NewTokenService(
		[]byte(cfg.JWT.Secret),
		"drexa.api",
		cfg.JWT.AccessExpiration,
		cfg.JWT.RefreshExpiration,
	)

	// ── Auth usecase ──────────────────────────────────────────────────────────
	authUsecase := authUc.NewAuthUsecase(userRepo, refreshTokenRepo, otpService, tokenService, cfg.Google.ClientID)

	// ── KYC domain ────────────────────────────────────────────────────────────
	kycRepository := kycRepo.New(db)
	kycUserSvc := &kycUserServiceAdapter{repo: userRepo}
	kycNotifSvc := kycSvc.NewMockNotificationService()
	kycUsecase := kycUc.New(kycRepository, kycUserSvc)
	adminKycUsecase := kycUc.NewAdmin(kycRepository, kycUserSvc, kycNotifSvc)

	getUserID := func(r *http.Request) string {
		claims, ok := auth.UserFromContext(r.Context())
		if !ok {
			return ""
		}
		return claims.UserID
	}
	kycHandler := kyc.NewHandler(kycUsecase, adminKycUsecase, getUserID)

	// ── Order domain ──────────────────────────────────────────────────────────
	orderRepository := orderRepo.New(db)
	pairService := orderRepo.NewPairService(db)
	matchingEngine := matching.NewEngine()
	orderService := order.NewService(orderRepository, pairService, matchingEngine)

	// ── Wallet domain ─────────────────────────────────────────────────────────
	walletRepository := walletRepo.NewWalletRepository(db)
	txRepository := walletRepo.NewTransactionRepository(db)
	depositRepository := walletRepo.NewDepositRepository(db)
	withdrawalRepository := walletRepo.NewWithdrawalRepository(db)
	cryptoAddressRepo := walletRepo.NewCryptoAddressRepository(db)
	paymentService := walletSvc.NewNullPaymentService()
	if cfg.Stripe.SecretKey != "" {
		paymentService = walletSvc.NewStripePaymentService(cfg.Stripe.SecretKey, cfg.SendGrid.AppURL)
	}
	cryptoProvider := walletSvc.NewTatumService(cfg.Tatum, "https://api.tatum.io")
	txManager := walletRepo.NewTxManager(db)

	walletUsecase := walletUc.NewWalletUsecase(walletRepository, txRepository, depositRepository, withdrawalRepository, paymentService, cryptoProvider, txManager)
	adminWalletUsecase := walletUc.NewAdminWalletUsecase(walletRepository, txRepository, withdrawalRepository, paymentService, txManager)
	cryptoWalletUsecase := walletUc.NewCryptoWalletUsecase(cryptoAddressRepo, walletRepository, txRepository, txManager, cryptoProvider, false)

	// ── Market data (real-time WebSocket feed) ─────────────────────────────────
	// The /market/ws feed now publishes our own order-book depth, sourced from
	// the in-memory matching engine, instead of the external Binance stream.
	marketHub := market.NewHub()
	go marketHub.Run()
	orderBookFeed := market.NewOrderBookFeed(
		marketHub,
		&depthSourceAdapter{orders: orderService},
		&pairListerAdapter{db: db},
	)
	go orderBookFeed.Run(context.Background())

	// ── Checkout domain ───────────────────────────────────────────────────────
	var checkoutHandler *checkout.Handler
	if cfg.Stripe.SecretKey != "" {
		purchaseRepo := checkout.NewPurchaseRepository(db)
		checkoutSvc := checkout.NewCheckoutService(
			cfg.Stripe.SecretKey,
			cfg.Stripe.WebhookSecret,
			cfg.SendGrid.AppURL, // Reusing AppURL as the base URL
			purchaseRepo,
			userRepo,
		)
		checkoutHandler = checkout.NewHandler(checkoutSvc, getUserID)
	}

	// ── HTTP ──────────────────────────────────────────────────────────────────
	mux := http.NewServeMux()
	addRoutes(mux, cfg, authUsecase, kycHandler, orderService, walletUsecase, adminWalletUsecase, cryptoWalletUsecase, marketHub, tokenService, checkoutHandler)

	handler := middleware.CORS(cfg.App.AllowedOrigins)(middleware.RequestID(mux))

	return &Server{
		httpServer: &http.Server{
			Addr:         cfg.App.Port,
			Handler:      handler,
			ReadTimeout:  cfg.App.ReadTimeout,
			WriteTimeout: cfg.App.WriteTimeout,
			IdleTimeout:  cfg.App.IdleTimeout,
		},
	}
}

func (s *Server) Start(ctx context.Context) error {
	ctx, cancel := signal.NotifyContext(ctx, os.Interrupt)
	defer cancel()

	log.Info().Str("addr", s.httpServer.Addr).Msg("server starting")

	go func() {
		if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			fmt.Fprintf(os.Stderr, "listen: %s\n", err)
		}
	}()

	<-ctx.Done()
	log.Info().Msg("shutdown signal received")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if err := s.httpServer.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("server shutdown: %w", err)
	}

	log.Info().Msg("server stopped cleanly")
	return nil
}
