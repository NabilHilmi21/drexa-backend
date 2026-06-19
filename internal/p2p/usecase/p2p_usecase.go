// Package usecase implements the P2P marketplace business logic. Crypto custody
// is delegated to the on-chain P2PEscrow contract (via p2p/chain): creating an
// order funds an escrow, the seller's confirmation releases it to the buyer, and
// cancel/expiry/dispute-resolution refund the seller — all on-chain.
package usecase

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"

	"drexa/internal/p2p"
	"drexa/internal/p2p/chain"
)

// service implements both p2p.Usecase and p2p.AdminUsecase.
type service struct {
	repo           p2p.Repository
	escrow         chain.EscrowClient
	confirmTimeout time.Duration
	walletSvc      p2p.WalletService
}

// New returns the user-facing P2P usecase.
func New(repo p2p.Repository, escrow chain.EscrowClient, confirmTimeout time.Duration, walletSvc p2p.WalletService) p2p.Usecase {
	return &service{repo: repo, escrow: escrow, confirmTimeout: confirmTimeout, walletSvc: walletSvc}
}

// NewAdmin returns the admin-facing P2P usecase (dispute resolution). It shares
// the same backing implementation as New.
func NewAdmin(repo p2p.Repository, escrow chain.EscrowClient, confirmTimeout time.Duration, walletSvc p2p.WalletService) p2p.AdminUsecase {
	return &service{repo: repo, escrow: escrow, confirmTimeout: confirmTimeout, walletSvc: walletSvc}
}

// chainCtx derives a context bounded by the configured tx-confirmation timeout.
func (s *service) chainCtx(ctx context.Context) (context.Context, context.CancelFunc) {
	if s.confirmTimeout <= 0 {
		return context.WithCancel(ctx)
	}
	return context.WithTimeout(ctx, s.confirmTimeout)
}

func ptr(s string) *string { return &s }

// ─── Advertisements ──────────────────────────────────────────────────────────

func (s *service) CreateAd(ctx context.Context, sellerID string, in p2p.CreateAdInput) (*p2p.P2PAdvertisement, error) {
	in.PairID = strings.TrimSpace(in.PairID)
	in.PaymentMethod = strings.TrimSpace(in.PaymentMethod)
	if in.PairID == "" || in.PaymentMethod == "" {
		return nil, p2p.ErrInvalidInput
	}
	if in.Price <= 0 || in.MinAmount <= 0 || in.MaxAmount < in.MinAmount {
		return nil, p2p.ErrInvalidInput
	}
	if in.PaymentWindow <= 0 {
		return nil, p2p.ErrInvalidInput
	}
	
	sellerAddress, err := s.walletSvc.GetDepositAddress(ctx, sellerID, "ETH")
	if err != nil {
		return nil, err
	}

	ad := &p2p.P2PAdvertisement{
		AdvertisementID: uuid.NewString(),
		SellerID:        sellerID,
		PairID:          in.PairID,
		Price:           in.Price,
		MinAmount:       in.MinAmount,
		MaxAmount:       in.MaxAmount,
		PaymentMethod:   in.PaymentMethod,
		PaymentWindow:   in.PaymentWindow,
		SellerAddress:   sellerAddress,
		Status:          p2p.AdStatusActive,
	}
	if err := s.repo.CreateAd(ctx, ad); err != nil {
		return nil, err
	}
	return ad, nil
}

func (s *service) ListAds(ctx context.Context, f p2p.AdFilter) ([]p2p.P2PAdvertisement, error) {
	// Public listings default to active ads only.
	if f.Status == "" {
		f.Status = p2p.AdStatusActive
	}
	return s.repo.ListAds(ctx, f)
}

func (s *service) GetAd(ctx context.Context, id string) (*p2p.P2PAdvertisement, error) {
	return s.repo.GetAd(ctx, id)
}

func (s *service) MyAds(ctx context.Context, sellerID string) ([]p2p.P2PAdvertisement, error) {
	return s.repo.ListAdsBySeller(ctx, sellerID)
}

func (s *service) SetAdStatus(ctx context.Context, sellerID, adID string, status p2p.AdvertisementStatus) error {
	switch status {
	case p2p.AdStatusActive, p2p.AdStatusPaused, p2p.AdStatusCompleted:
	default:
		return p2p.ErrInvalidInput
	}
	ad, err := s.repo.GetAd(ctx, adID)
	if err != nil {
		return err
	}
	if ad.SellerID != sellerID {
		return p2p.ErrForbidden
	}
	return s.repo.UpdateAdStatus(ctx, adID, status)
}

// ─── Orders ──────────────────────────────────────────────────────────────────

func (s *service) CreateOrder(ctx context.Context, buyerID string, in p2p.CreateOrderInput) (*p2p.P2POrder, error) {
	if in.AdvertisementID == "" || in.Amount <= 0 {
		return nil, p2p.ErrInvalidInput
	}
	
	buyerAddress, err := s.walletSvc.GetDepositAddress(ctx, buyerID, "ETH")
	if err != nil {
		return nil, err
	}

	ad, err := s.repo.GetAd(ctx, in.AdvertisementID)
	if err != nil {
		return nil, err
	}
	if ad.Status != p2p.AdStatusActive {
		return nil, p2p.ErrAdNotActive
	}
	if ad.SellerID == buyerID {
		return nil, p2p.ErrSelfTrade
	}
	if in.Amount < ad.MinAmount || in.Amount > ad.MaxAmount {
		return nil, p2p.ErrAmountOutOfRange
	}
	if !chain.IsAddress(ad.SellerAddress) {
		// Misconfigured ad — seller never set a payout address.
		return nil, p2p.ErrInvalidAddress
	}

	parts := strings.Split(ad.PairID, "-")
	baseCurrency := parts[0]

	orderID := uuid.NewString()

	// Deduct the crypto from the seller's internal asset ledger
	if err := s.walletSvc.DebitBalance(ctx, ad.SellerID, baseCurrency, in.Amount, orderID, "P2P Escrow Funding"); err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	order := &p2p.P2POrder{
		P2POrderID:      orderID,
		AdvertisementID: ad.AdvertisementID,
		BuyerID:         buyerID,
		SellerID:        ad.SellerID,
		Amount:          in.Amount,
		TotalIDR:        in.Amount * ad.Price,
		Status:          p2p.P2POrderCreated,
		BuyerAddress:    buyerAddress,
		SellerAddress:   ad.SellerAddress,
		OnChainID:       s.escrow.OrderHash(orderID),
		EscrowState:     chain.StateNone.String(),
		ExpiredAt:       now.Add(time.Duration(ad.PaymentWindow) * time.Minute),
	}
	if err := s.repo.CreateOrder(ctx, order); err != nil {
		return nil, err
	}

	// Fund the on-chain escrow with the seller's crypto (platform-operated).
	cctx, cancel := s.chainCtx(ctx)
	defer cancel()
	txHash, err := s.escrow.CreateEscrow(cctx, orderID, order.BuyerAddress, order.SellerAddress, chain.EthToWei(in.Amount))
	if err != nil {
		// Refund the seller's internal balance because the escrow creation failed.
		_ = s.walletSvc.CreditBalance(ctx, ad.SellerID, baseCurrency, in.Amount, orderID, "Refund: P2P Escrow Funding Failed")

		// Roll the order back to cancelled so it can't be acted on.
		order.Status = p2p.P2POrderCancelled
		_ = s.repo.UpdateOrder(ctx, order)
		return nil, err
	}

	order.CreateTxHash = &txHash
	order.EscrowState = chain.StateFunded.String()
	if err := s.repo.UpdateOrder(ctx, order); err != nil {
		return nil, err
	}
	return order, nil
}

func (s *service) MarkPaid(ctx context.Context, userID, orderID string, proofURL *string) (*p2p.P2POrder, error) {
	order, err := s.repo.GetOrder(ctx, orderID)
	if err != nil {
		return nil, err
	}
	if order.BuyerID != userID {
		return nil, p2p.ErrForbidden
	}
	if order.Status != p2p.P2POrderCreated {
		return nil, p2p.ErrInvalidState
	}
	if time.Now().UTC().After(order.ExpiredAt) {
		return nil, p2p.ErrP2POrderExpired
	}

	// Record the fiat-paid flag on-chain (informational).
	cctx, cancel := s.chainCtx(ctx)
	defer cancel()
	txHash, err := s.escrow.MarkPaid(cctx, orderID)
	if err != nil {
		return nil, err
	}
	_ = txHash // state transition is captured below; tx hash is incidental here

	now := time.Now().UTC()
	order.Status = p2p.P2POrderPaid
	order.EscrowState = chain.StatePaid.String()
	order.PaidAt = &now
	if proofURL != nil && strings.TrimSpace(*proofURL) != "" {
		order.PaymentProofURL = proofURL
	}
	if err := s.repo.UpdateOrder(ctx, order); err != nil {
		return nil, err
	}
	return order, nil
}

func (s *service) ReleaseOrder(ctx context.Context, userID, orderID string) (*p2p.P2POrder, error) {
	order, err := s.repo.GetOrder(ctx, orderID)
	if err != nil {
		return nil, err
	}
	if order.SellerID != userID {
		return nil, p2p.ErrForbidden
	}
	if order.Status != p2p.P2POrderCreated && order.Status != p2p.P2POrderPaid {
		return nil, p2p.ErrInvalidState
	}

	cctx, cancel := s.chainCtx(ctx)
	defer cancel()
	txHash, err := s.escrow.Release(cctx, orderID)
	if err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	order.Status = p2p.P2POrderReleased
	order.EscrowState = chain.StateReleased.String()
	order.ReleasedAt = &now
	order.ReleaseTxHash = &txHash
	if err := s.repo.UpdateOrder(ctx, order); err != nil {
		return nil, err
	}
	return order, nil
}

func (s *service) CancelOrder(ctx context.Context, userID, orderID string) (*p2p.P2POrder, error) {
	order, err := s.repo.GetOrder(ctx, orderID)
	if err != nil {
		return nil, err
	}
	if order.BuyerID != userID && order.SellerID != userID {
		return nil, p2p.ErrForbidden
	}
	// Only cancellable before the buyer has marked payment — refunds the seller.
	if order.Status != p2p.P2POrderCreated {
		return nil, p2p.ErrInvalidState
	}

	cctx, cancel := s.chainCtx(ctx)
	defer cancel()
	txHash, err := s.escrow.Refund(cctx, orderID)
	if err != nil {
		return nil, err
	}

	order.Status = p2p.P2POrderCancelled
	order.EscrowState = chain.StateRefunded.String()
	order.RefundTxHash = &txHash
	if err := s.repo.UpdateOrder(ctx, order); err != nil {
		return nil, err
	}
	return order, nil
}

func (s *service) GetOrder(ctx context.Context, userID, orderID string) (*p2p.P2POrder, error) {
	order, err := s.repo.GetOrder(ctx, orderID)
	if err != nil {
		return nil, err
	}
	if order.BuyerID != userID && order.SellerID != userID {
		return nil, p2p.ErrForbidden
	}
	return order, nil
}

func (s *service) MyOrders(ctx context.Context, userID string) ([]p2p.P2POrder, error) {
	return s.repo.ListOrdersByUser(ctx, userID)
}

func (s *service) EscrowInfo(ctx context.Context, userID, orderID string) (p2p.OnChainEscrow, error) {
	order, err := s.repo.GetOrder(ctx, orderID)
	if err != nil {
		return p2p.OnChainEscrow{}, err
	}
	if order.BuyerID != userID && order.SellerID != userID {
		return p2p.OnChainEscrow{}, p2p.ErrForbidden
	}

	cctx, cancel := s.chainCtx(ctx)
	defer cancel()
	info, err := s.escrow.GetEscrow(cctx, orderID)
	if err != nil {
		return p2p.OnChainEscrow{}, err
	}
	amount := "0"
	if info.Amount != nil {
		amount = info.Amount.String()
	}
	return p2p.OnChainEscrow{
		Buyer:     info.Buyer,
		Seller:    info.Seller,
		AmountWei: amount,
		State:     info.State.String(),
		CreatedAt: info.CreatedAt,
	}, nil
}

// ─── Disputes ────────────────────────────────────────────────────────────────

func (s *service) OpenDispute(ctx context.Context, userID, orderID string, in p2p.OpenDisputeInput) (*p2p.P2PDispute, error) {
	in.Reason = strings.TrimSpace(in.Reason)
	if in.Reason == "" {
		return nil, p2p.ErrInvalidInput
	}

	order, err := s.repo.GetOrder(ctx, orderID)
	if err != nil {
		return nil, err
	}
	if order.BuyerID != userID && order.SellerID != userID {
		return nil, p2p.ErrForbidden
	}
	if order.Status != p2p.P2POrderCreated && order.Status != p2p.P2POrderPaid {
		return nil, p2p.ErrInvalidState
	}

	// Reject if a dispute is already open for this order.
	if existing, err := s.repo.GetDisputeByOrder(ctx, orderID); err == nil && existing.Status == p2p.DisputeOpen {
		return nil, p2p.ErrDisputeExists
	} else if err != nil && !errors.Is(err, p2p.ErrDisputeNotFound) {
		return nil, err
	}

	// Freeze the escrow on-chain pending arbiter resolution.
	cctx, cancel := s.chainCtx(ctx)
	defer cancel()
	txHash, err := s.escrow.RaiseDispute(cctx, orderID)
	if err != nil {
		return nil, err
	}

	order.Status = p2p.P2POrderDisputed
	order.EscrowState = chain.StateDisputed.String()
	order.DisputeTxHash = &txHash
	if err := s.repo.UpdateOrder(ctx, order); err != nil {
		return nil, err
	}

	dispute := &p2p.P2PDispute{
		P2PDisputeID: uuid.NewString(),
		P2POrderID:   orderID,
		RaisedBy:     userID,
		Reason:       in.Reason,
		EvidenceURL:  in.EvidenceURL,
		Status:       p2p.DisputeOpen,
	}
	if err := s.repo.CreateDispute(ctx, dispute); err != nil {
		return nil, err
	}
	return dispute, nil
}

// ─── Admin ───────────────────────────────────────────────────────────────────

func (s *service) ListOpenDisputes(ctx context.Context) ([]p2p.P2PDispute, error) {
	return s.repo.ListOpenDisputes(ctx)
}

func (s *service) ResolveDispute(ctx context.Context, adminID, disputeID string, releaseToBuyer bool, resolution string) (*p2p.P2PDispute, error) {
	dispute, err := s.repo.GetDispute(ctx, disputeID)
	if err != nil {
		return nil, err
	}
	if dispute.Status != p2p.DisputeOpen {
		return nil, p2p.ErrInvalidState
	}
	order, err := s.repo.GetOrder(ctx, dispute.P2POrderID)
	if err != nil {
		return nil, err
	}

	// Arbiter resolves on-chain: funds go to the buyer (release) or seller (refund).
	cctx, cancel := s.chainCtx(ctx)
	defer cancel()
	txHash, err := s.escrow.ResolveDispute(cctx, order.P2POrderID, releaseToBuyer)
	if err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	if releaseToBuyer {
		order.Status = p2p.P2POrderReleased
		order.EscrowState = chain.StateReleased.String()
		order.ReleasedAt = &now
		order.ReleaseTxHash = &txHash
	} else {
		order.Status = p2p.P2POrderCancelled
		order.EscrowState = chain.StateRefunded.String()
		order.RefundTxHash = &txHash
	}
	if err := s.repo.UpdateOrder(ctx, order); err != nil {
		return nil, err
	}

	dispute.Status = p2p.DisputeResolved
	dispute.ResolvedBy = ptr(adminID)
	dispute.Resolution = resolution
	dispute.ResolvedAt = &now
	if err := s.repo.UpdateDispute(ctx, dispute); err != nil {
		return nil, err
	}
	return dispute, nil
}
