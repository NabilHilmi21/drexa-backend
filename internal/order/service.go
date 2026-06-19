package order

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"drexa/internal/matching"
)

type service struct {
	repo       Repository
	pairs      PairService
	matcher    Matcher
	walletSvc  WalletService
}

// NewService wires the order service with its persistence, the market-backed
// trading-pair lookup, the in-memory matching engine, and wallet balance checks.
func NewService(repo Repository, pairs PairService, matcher Matcher, walletSvc WalletService) Service {
	return &service{repo: repo, pairs: pairs, matcher: matcher, walletSvc: walletSvc}
}

// smallestUnitFactor returns how many of the currency's base units equal one
// whole unit. Used to convert float order amounts into the int64 stored by wallets.
func smallestUnitFactor(currency string) int64 {
	switch currency {
	case "BTC":
		return 100_000_000
	case "ETH", "BNB":
		return 1_000_000_000_000_000_000
	case "SOL":
		return 1_000_000_000
	case "USDT":
		return 1_000_000
	default: // USD, IDR
		return 100
	}
}

// CreateOrder validates the request, persists the order in a pending state,
// then runs it through the matching engine. Resulting trades are persisted and
// every affected order's fill/status is updated.
//
// Settlement (ledger debit/credit, fee capture) is Fase 4A.3 and not done here;
// fees are recorded as zero.
func (s *service) CreateOrder(ctx context.Context, userID string, req OrderRequest) (*Order, error) {
	if req.Side != SideBuy && req.Side != SideSell {
		return nil, ErrInvalidSide
	}

	switch req.Type {
	case TypeMarket:
		if req.Price != nil {
			return nil, ErrPriceNotAllowed
		}
	case TypeLimit:
		if req.Price == nil || *req.Price <= 0 {
			return nil, ErrPriceRequired
		}
	default:
		return nil, ErrInvalidType
	}

	pair, err := s.pairs.GetPair(ctx, req.PairID)
	if err != nil {
		return nil, err
	}
	if !pair.Active {
		return nil, ErrPairSuspended
	}
	if req.Quantity < pair.MinOrderSize {
		return nil, ErrBelowMinOrderSize
	}

	// For limit orders: validate that the user holds enough balance before
	// persisting. Market orders are priced at match time so we skip the check.
	if req.Type == TypeLimit {
		var checkCurrency string
		var checkAmount float64
		switch req.Side {
		case SideBuy:
			checkCurrency = pair.QuoteCoin
			checkAmount = req.Quantity * *req.Price
		case SideSell:
			checkCurrency = pair.BaseCoin
			checkAmount = req.Quantity
		}
		available, err := s.walletSvc.AvailableBalance(ctx, userID, checkCurrency)
		if err != nil {
			return nil, fmt.Errorf("balance check: %w", err)
		}
		needed := int64(checkAmount * float64(smallestUnitFactor(checkCurrency)))
		if available < needed {
			return nil, ErrInsufficientBalance
		}
	}

	o := &Order{
		OrderID:  uuid.NewString(),
		UserID:   userID,
		PairID:   req.PairID,
		Side:     req.Side,
		Type:     req.Type,
		Status:   StatusPending,
		Price:    req.Price,
		Quantity: req.Quantity,
	}

	// For limit orders the amount to lock is deterministic: buyers lock quote
	// currency (quantity × price), sellers lock base currency (quantity).
	// Market orders are priced at match time, so nothing is locked up front.
	if req.Type == TypeLimit {
		switch req.Side {
		case SideBuy:
			o.LockedAmount = req.Quantity * *req.Price
		case SideSell:
			o.LockedAmount = req.Quantity
		}
	}

	if err := s.repo.Create(ctx, o); err != nil {
		return nil, err
	}

	// Match against the live book for this pair.
	result := s.matcher.Submit(req.PairID, toEngineOrder(o, pair.PriceDecimals))

	if err := s.applyResult(ctx, o, result, pair.PriceDecimals); err != nil {
		return nil, err
	}

	return o, nil
}

// applyResult persists the fills and the new state of every order touched by a
// match: the taker (o, in memory), every maker that traded, and any maker
// canceled by self-trade prevention.
func (s *service) applyResult(ctx context.Context, taker *Order, result matching.MatchResult, priceDec int) error {
	if len(result.Trades) > 0 {
		trades := make([]Trade, 0, len(result.Trades))
		now := time.Now()
		for _, t := range result.Trades {
			trades = append(trades, Trade{
				TradeID:      uuid.NewString(),
				PairID:       taker.PairID,
				MakerOrderID: t.MakerOrderID,
				TakerOrderID: t.TakerOrderID,
				Price:        ticksToPrice(t.Price, priceDec),
				Quantity:     lotsToQty(t.Quantity),
				ExecutedAt:   now,
			})
		}
		if err := s.repo.SaveTrades(ctx, trades); err != nil {
			return err
		}
	}

	// Sum this round's fill delta per order id (in float, pair units).
	fillDelta := make(map[string]float64)
	for _, t := range result.Trades {
		qty := lotsToQty(t.Quantity)
		fillDelta[t.MakerOrderID] += qty
		fillDelta[t.TakerOrderID] += qty
	}

	// Update each resting maker that traded (skip the taker; handled below).
	for id, delta := range fillDelta {
		if id == taker.OrderID {
			continue
		}
		maker, err := s.repo.FindByID(ctx, id)
		if err != nil {
			return err
		}
		maker.FilledQuantity += delta
		maker.Status = deriveStatus(maker, true) // a traded maker either filled out or still rests
		if err := s.repo.Update(ctx, maker); err != nil {
			return err
		}
	}

	// Makers removed by self-trade prevention are canceled.
	for _, c := range result.Canceled {
		maker, err := s.repo.FindByID(ctx, c.ID)
		if err != nil {
			return err
		}
		maker.Status = StatusCancelled
		if err := s.repo.Update(ctx, maker); err != nil {
			return err
		}
	}

	// Finally the taker itself.
	taker.FilledQuantity += fillDelta[taker.OrderID]
	taker.Status = deriveStatus(taker, result.Rested)
	return s.repo.Update(ctx, taker)
}

// OrderBookDepth returns the live aggregated book for a pair, converting the
// engine's integer ticks/lots back into real prices using the pair's quoted
// precision.
func (s *service) OrderBookDepth(ctx context.Context, pairID string, maxLevels int) (*OrderBookSnapshot, error) {
	pair, err := s.pairs.GetPair(ctx, pairID)
	if err != nil {
		return nil, err
	}

	depth := s.matcher.Depth(pairID, maxLevels)
	snap := &OrderBookSnapshot{
		PairID:  pairID,
		Version: depth.Version,
		Bids:    toBookLevels(depth.Bids, pair.PriceDecimals),
		Asks:    toBookLevels(depth.Asks, pair.PriceDecimals),
	}
	return snap, nil
}

// toBookLevels converts engine tick/lot levels into float price levels.
func toBookLevels(levels []matching.DepthLevel, priceDec int) []OrderBookLevel {
	out := make([]OrderBookLevel, len(levels))
	for i, l := range levels {
		out[i] = OrderBookLevel{
			Price:    ticksToPrice(l.Price, priceDec),
			Quantity: lotsToQty(l.Volume),
		}
	}
	return out
}

func (s *service) ListOrders(ctx context.Context, userID, status, pairID string, limit, offset int) ([]Order, int64, error) {
	return s.repo.FindByUserIDFiltered(ctx, userID, status, pairID, limit, offset)
}

func (s *service) ListTrades(ctx context.Context, userID, pairID string, limit, offset int) ([]UserTrade, int64, error) {
	return s.repo.FindTradesByUserID(ctx, userID, pairID, limit, offset)
}

// CancelOrder removes a still-open order from the book and marks it cancelled.
func (s *service) CancelOrder(ctx context.Context, userID, orderID string) (*Order, error) {
	o, err := s.repo.FindByID(ctx, orderID)
	if err != nil {
		return nil, err
	}
	// Don't disclose existence of another user's order.
	if o.UserID != userID {
		return nil, ErrOrderNotFound
	}

	switch o.Status {
	case StatusPending, StatusOpen, StatusPartiallyFilled:
		// cancellable
	default:
		return nil, ErrOrderNotCancellable
	}

	if _, err := s.matcher.Cancel(o.PairID, orderID); err != nil {
		// Not resting on the book (already fully filled/canceled in the engine).
		return nil, ErrOrderNotCancellable
	}

	o.Status = StatusCancelled
	if err := s.repo.Update(ctx, o); err != nil {
		return nil, err
	}
	return o, nil
}
