package service

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"fmt"
	"math/big"
	"time"

	"github.com/redis/go-redis/v9"

	"drexa/internal/auth"
)

const (
	otpRedisPrefix    = "otp:"
	otpAttemptsPrefix = "otp:attempts:"
	otpRedisMaxTTL    = 10 * time.Minute
	otpMaxAttempts    = 5
)

type redisOTPService struct {
	rdb   *redis.Client
	email EmailSender
	sms   SMSSender
}

// NewRedisOTPService returns an OTPService backed by Redis for storage
// and SendGrid/Twilio for delivery.
func NewRedisOTPService(rdb *redis.Client, email EmailSender, sms SMSSender) auth.OTPService {
	return &redisOTPService{rdb: rdb, email: email, sms: sms}
}

func (s *redisOTPService) GenerateAndSendEmail(ctx context.Context, key, email string) error {
	code, err := generateOTP()
	if err != nil {
		return fmt.Errorf("otp_redis: generate code: %w", err)
	}

	if err := s.store(ctx, key, code); err != nil {
		return err
	}

	subject := "Your Drexa verification code"
	body := fmt.Sprintf("Your verification code is: %s\n\nThis code expires in 10 minutes.", code)
	return s.email.SendEmail(ctx, email, subject, body)
}

func (s *redisOTPService) GenerateAndSendSMS(ctx context.Context, key, phone string) error {
	code, err := generateOTP()
	if err != nil {
		return fmt.Errorf("otp_redis: generate code: %w", err)
	}

	if err := s.store(ctx, key, code); err != nil {
		return err
	}

	body := fmt.Sprintf("Your Drexa verification code: %s (expires in 10 min)", code)
	return s.sms.SendSMS(ctx, phone, body)
}

func (s *redisOTPService) Verify(ctx context.Context, key, otp string) (bool, error) {
	stored, err := s.rdb.Get(ctx, otpRedisPrefix+key).Result()
	if err == redis.Nil {
		return false, nil // no OTP for this key (expired or already consumed)
	}
	if err != nil {
		return false, fmt.Errorf("otp_redis: redis get: %w", err)
	}

	// Brute-force protection: track failed attempts per key
	attemptsKey := otpAttemptsPrefix + key
	attempts, err := s.rdb.Incr(ctx, attemptsKey).Result()
	if err != nil {
		return false, fmt.Errorf("otp_redis: track attempts: %w", err)
	}
	if attempts == 1 {
		// Align counter TTL with the OTP window
		s.rdb.Expire(ctx, attemptsKey, otpRedisMaxTTL)
	}
	if attempts > otpMaxAttempts {
		// Too many wrong guesses — invalidate the OTP immediately
		s.rdb.Del(ctx, otpRedisPrefix+key)
		return false, auth.ErrOTPInvalid
	}

	// Constant-time compare prevents timing attacks
	match := subtle.ConstantTimeCompare([]byte(stored), []byte(otp)) == 1
	if !match {
		return false, nil
	}

	// Consume OTP and clear attempt counter on success
	s.rdb.Del(ctx, otpRedisPrefix+key)
	s.rdb.Del(ctx, attemptsKey)
	return true, nil
}

func (s *redisOTPService) store(ctx context.Context, key, code string) error {
	// Reset attempt counter whenever a fresh OTP is issued for this key
	s.rdb.Del(ctx, otpAttemptsPrefix+key)
	if err := s.rdb.Set(ctx, otpRedisPrefix+key, code, otpRedisMaxTTL).Err(); err != nil {
		return fmt.Errorf("otp_redis: redis set: %w", err)
	}
	return nil
}

func generateOTP() (string, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(1_000_000))
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%06d", n.Int64()), nil
}
