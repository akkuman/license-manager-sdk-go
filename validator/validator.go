package validator

import (
	"crypto"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/cedar-v/license-manage-sdk-go/models"
)

// Validator verifies license signatures & constraints.
type Validator struct {
	pub *rsa.PublicKey
	extraVerify func(*models.LicensePayload) error
}

// New creates a Validator from PEM encoded RSA public key.
func New(pemBytes []byte, extraVerify func(*models.LicensePayload) error) (*Validator, error) {
	if len(pemBytes) == 0 {
		return nil, errors.New("validator: missing public key")
	}
	if extraVerify == nil {
		extraVerify = func(payload *models.LicensePayload) error {
			if !payload.ExpiresAt.IsZero() && time.Now().After(payload.ExpiresAt) {
				return errors.New("validator: license expired")
			}
			return nil
		}
	}
	block, _ := pem.Decode(pemBytes)
	if block == nil {
		return nil, errors.New("validator: invalid PEM")
	}
	pubAny, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("validator: parse public key: %w", err)
	}
	pub, ok := pubAny.(*rsa.PublicKey)
	if !ok {
		return nil, errors.New("validator: public key is not RSA")
	}
	return &Validator{
		pub: pub,
		extraVerify: extraVerify,
	}, nil
}

// Verify checks signature, expiration and hardware binding.
func (v *Validator) Verify(envelopeBytes []byte, fingerprint string) (*models.LicensePayload, error) {
	var env models.LicenseEnvelope
	if err := json.Unmarshal(envelopeBytes, &env); err != nil {
		return nil, fmt.Errorf("validator: decode license: %w", err)
	}
	payloadBytes := []byte(env.Data)
	sig, err := base64.StdEncoding.DecodeString(env.Signature)
	if err != nil {
		return nil, fmt.Errorf("validator: decode signature: %w", err)
	}
	hash := sha256.Sum256(payloadBytes)
	if err := v.verifySignature(strings.ToUpper(env.Algorithm), hash[:], sig); err != nil {
		return nil, err
	}
	var payload models.LicensePayload
	if err := json.Unmarshal(payloadBytes, &payload); err != nil {
		return nil, fmt.Errorf("validator: decode payload: %w", err)
	}
	if err := json.Unmarshal(payloadBytes, &payload.Extras); err != nil {
		payload.Extras = nil
	}
	if !payload.EndDate.IsZero() {
		payload.ExpiresAt = payload.EndDate
	}
	if fingerprint != "" && payload.HardwareFingerprint != "" && payload.HardwareFingerprint != fingerprint {
		return nil, errors.New("validator: hardware fingerprint mismatch")
	}
	if err := v.extraVerify(&payload); err != nil {
		return nil, err
	}
	return &payload, nil
}

func (v *Validator) verifySignature(algorithm string, hash []byte, sig []byte) error {
	switch algorithm {
	case "", "RSA-SHA256", "RSA-PKCS1V15-SHA256":
		if err := rsa.VerifyPKCS1v15(v.pub, crypto.SHA256, hash, sig); err != nil {
			return fmt.Errorf("validator: signature mismatch: %w", err)
		}
		return nil
	case "RSA-PSS-SHA256":
		if err := rsa.VerifyPSS(v.pub, crypto.SHA256, hash, sig, &rsa.PSSOptions{SaltLength: rsa.PSSSaltLengthEqualsHash, Hash: crypto.SHA256}); err != nil {
			return fmt.Errorf("validator: signature mismatch: %w", err)
		}
		return nil
	default:
		if err := rsa.VerifyPKCS1v15(v.pub, crypto.SHA256, hash, sig); err != nil {
			return fmt.Errorf("validator: unsupported algorithm %s: %w", algorithm, err)
		}
		return nil
	}
}
