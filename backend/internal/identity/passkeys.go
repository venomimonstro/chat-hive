package identity

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/webauthn"
)

var (
	ErrPasskeyInvalidCeremony = errors.New("invalid or expired passkey ceremony")
	ErrPasskeyUnavailable     = errors.New("passkey unavailable")
)

type PasskeyInfo struct {
	CredentialID string     `json:"credential_id"`
	Label        string     `json:"label"`
	CreatedAt    time.Time  `json:"created_at"`
	LastUsedAt   *time.Time `json:"last_used_at,omitempty"`
}

type passkeyUser struct {
	ID          string
	Name        string
	DisplayName string
	Credentials []webauthn.Credential
}

func (u passkeyUser) WebAuthnID() []byte                         { return []byte(u.ID) }
func (u passkeyUser) WebAuthnName() string                      { return u.Name }
func (u passkeyUser) WebAuthnDisplayName() string               { return u.DisplayName }
func (u passkeyUser) WebAuthnCredentials() []webauthn.Credential { return u.Credentials }

type PasskeyService struct {
	store    *PostgresStore
	identity *Service
	web      *webauthn.WebAuthn
}

func NewPasskeyService(store *PostgresStore, identity *Service, webOrigin string) (*PasskeyService, error) {
	parsed, err := url.Parse(strings.TrimSpace(webOrigin))
	if err != nil || parsed.Hostname() == "" {
		return nil, fmt.Errorf("invalid passkey origin")
	}
	web, err := webauthn.New(&webauthn.Config{
		RPDisplayName: "CHAT",
		RPID:          parsed.Hostname(),
		RPOrigins:     []string{strings.TrimRight(webOrigin, "/")},
		AttestationPreference: protocol.PreferNoAttestation,
		AuthenticatorSelection: protocol.AuthenticatorSelection{
			ResidentKey:      protocol.ResidentKeyRequirementRequired,
			RequireResidentKey: protocol.ResidentKeyRequired(),
			UserVerification: protocol.VerificationRequired,
		},
		Timeouts: webauthn.TimeoutsConfig{
			Registration: webauthn.TimeoutConfig{Enforce: true, Timeout: 2 * time.Minute, TimeoutUVD: 2 * time.Minute},
			Login:        webauthn.TimeoutConfig{Enforce: true, Timeout: 2 * time.Minute, TimeoutUVD: 2 * time.Minute},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("configure WebAuthn: %w", err)
	}
	return &PasskeyService{store: store, identity: identity, web: web}, nil
}

func (s *PasskeyService) BeginRegistration(ctx context.Context, userID string) (*protocol.CredentialCreation, string, error) {
	user, err := s.store.LoadPasskeyUser(ctx, userID)
	if err != nil {
		return nil, "", err
	}
	exclusions := make([]protocol.CredentialDescriptor, 0, len(user.Credentials))
	for _, credential := range user.Credentials {
		exclusions = append(exclusions, credential.Descriptor())
	}
	options, session, err := s.web.BeginRegistration(user,
		webauthn.WithResidentKeyRequirement(protocol.ResidentKeyRequirementRequired),
		webauthn.WithConveyancePreference(protocol.PreferNoAttestation),
		webauthn.WithExclusions(exclusions),
	)
	if err != nil {
		return nil, "", fmt.Errorf("begin passkey registration: %w", err)
	}
	ceremonyID, err := s.store.CreatePasskeyCeremony(ctx, "registration", userID, *session)
	if err != nil {
		return nil, "", err
	}
	return options, ceremonyID, nil
}

func (s *PasskeyService) FinishRegistration(ctx context.Context, userID, ceremonyID string, r *http.Request) error {
	storedUserID, session, err := s.store.ConsumePasskeyCeremony(ctx, ceremonyID, "registration", time.Now().UTC())
	if err != nil {
		return err
	}
	if storedUserID == "" || storedUserID != userID {
		return ErrPasskeyInvalidCeremony
	}
	user, err := s.store.LoadPasskeyUser(ctx, userID)
	if err != nil {
		return err
	}
	credential, err := s.web.FinishRegistration(user, session, r)
	if err != nil {
		return fmt.Errorf("finish passkey registration: %w", err)
	}
	if err := s.store.SavePasskeyCredential(ctx, userID, *credential); err != nil {
		return err
	}
	return nil
}

func (s *PasskeyService) BeginLogin(ctx context.Context) (*protocol.CredentialAssertion, string, error) {
	options, session, err := s.web.BeginDiscoverableLogin(
		webauthn.WithUserVerification(protocol.VerificationRequired),
	)
	if err != nil {
		return nil, "", fmt.Errorf("begin passkey login: %w", err)
	}
	ceremonyID, err := s.store.CreatePasskeyCeremony(ctx, "login", "", *session)
	if err != nil {
		return nil, "", err
	}
	return options, ceremonyID, nil
}

func (s *PasskeyService) FinishLogin(ctx context.Context, ceremonyID string, r *http.Request, userAgent, requestIP string) (SessionTokens, error) {
	_, session, err := s.store.ConsumePasskeyCeremony(ctx, ceremonyID, "login", time.Now().UTC())
	if err != nil {
		return SessionTokens{}, err
	}

	var resolved *passkeyUser
	handler := func(rawID, userHandle []byte) (webauthn.User, error) {
		user, err := s.store.LoadPasskeyUserByHandle(ctx, userHandle)
		if err != nil {
			return nil, err
		}
		resolved = &user
		return user, nil
	}

	credential, err := s.web.FinishDiscoverableLogin(handler, session, r)
	if err != nil {
		return SessionTokens{}, fmt.Errorf("finish passkey login: %w", err)
	}
	if resolved == nil {
		return SessionTokens{}, ErrPasskeyUnavailable
	}
	if err := s.store.UpdatePasskeyCredential(ctx, resolved.ID, *credential); err != nil {
		return SessionTokens{}, err
	}
	if credential.Authenticator.CloneWarning {
		if err := s.store.RecordPasskeyCloneWarning(ctx, resolved.ID, credential.ID, requestIP); err != nil {
			return SessionTokens{}, err
		}
		return SessionTokens{}, ErrPasskeyUnavailable
	}
	return s.identity.CreateSessionForUser(ctx, resolved.ID, userAgent, requestIP)
}


func (s *PasskeyService) ListCredentials(ctx context.Context, userID string) ([]PasskeyInfo, error) {
	return s.store.ListPasskeyInfo(ctx, userID)
}

func (s *PasskeyService) DeleteCredential(ctx context.Context, userID, encodedCredentialID string) error {
	return s.store.DeletePasskeyCredential(ctx, userID, encodedCredentialID)
}
