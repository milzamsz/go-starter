package auth

import (
	"fmt"
	"time"

	"github.com/pquerna/otp/totp"
)

// GenerateTOTPSecret creates a new TOTP secret and returns the secret string
// and an otpauth:// provisioning URI suitable for QR code generation.
func GenerateTOTPSecret(email, issuer string) (secret string, qrURL string, err error) {
	key, err := totp.Generate(totp.GenerateOpts{
		Issuer:      issuer,
		AccountName: email,
		Period:      30,
		Digits:      6,
	})
	if err != nil {
		return "", "", fmt.Errorf("generating TOTP key: %w", err)
	}

	return key.Secret(), key.URL(), nil
}

// ValidateTOTPCode validates a TOTP code against the given secret.
// It allows a ±1 time-step skew to account for clock drift.
func ValidateTOTPCode(secret, code string) bool {
	valid, _ := totp.ValidateCustom(code, secret, time.Now(), totp.ValidateOpts{
		Period:    30,
		Skew:     1,
		Digits:   6,
	})
	return valid
}
