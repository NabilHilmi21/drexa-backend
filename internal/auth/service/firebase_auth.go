package service

import (
	"context"
	"errors"

	firebaseauth "firebase.google.com/go/v4/auth"

	"drexa/internal/auth"
)

// FirebaseAuthService verifies Firebase ID tokens from the frontend SDK.
type FirebaseAuthService struct {
	client *firebaseauth.Client
}

// NewFirebaseAuthService constructs a FirebaseAuthService using the initialized Firebase auth client.
func NewFirebaseAuthService(client *firebaseauth.Client) *FirebaseAuthService {
	return &FirebaseAuthService{client: client}
}

// VerifyIDToken verifies a Firebase ID token and returns the extracted claims.
func (f *FirebaseAuthService) VerifyIDToken(ctx context.Context, idToken string) (*auth.FirebaseClaims, error) {
	token, err := f.client.VerifyIDToken(ctx, idToken)
	if err != nil {
		return nil, err
	}

	claims := &auth.FirebaseClaims{
		UID:      token.UID,
		Provider: token.Firebase.SignInProvider,
	}

	if email, ok := token.Claims["email"].(string); ok {
		claims.Email = email
	}
	if verified, ok := token.Claims["email_verified"].(bool); ok {
		claims.EmailVerified = verified
	}

	return claims, nil
}

// nullFirebaseVerifier is returned when Firebase is not configured.
// It rejects every ID token with a clear error so OAuth endpoints fail gracefully.
type nullFirebaseVerifier struct{}

// NewNullFirebaseVerifier returns a FirebaseVerifier that always returns an error.
// Use this when FIREBASE_CREDENTIALS_JSON is not set.
func NewNullFirebaseVerifier() auth.FirebaseVerifier {
	return &nullFirebaseVerifier{}
}

func (n *nullFirebaseVerifier) VerifyIDToken(_ context.Context, _ string) (*auth.FirebaseClaims, error) {
	return nil, errors.New("firebase not configured: set FIREBASE_CREDENTIALS_JSON in .env")
}
