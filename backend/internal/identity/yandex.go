package identity

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

var ErrOAuthState = errors.New("invalid or expired oauth state")

type YandexConfig struct {
	ClientID       string
	RedirectURL    string
	WebCompleteURL string
}

type YandexStore interface {
	CreateOAuthState(ctx context.Context, provider string, stateHash []byte, verifier, requestIP string, expiresAt time.Time) error
	ConsumeOAuthState(ctx context.Context, provider string, stateHash []byte, now time.Time) (string, error)
	ResolveYandexIdentity(ctx context.Context, subject, email string, now time.Time) (string, error)
	CreateSession(ctx context.Context, input CreateSessionInput) (string, error)
}

type YandexOAuth struct {
	store  YandexStore
	client *http.Client
	config YandexConfig
	now    func() time.Time
}

func NewYandexOAuth(store YandexStore, config YandexConfig) *YandexOAuth {
	return &YandexOAuth{
		store: store,
		client: &http.Client{Timeout: 8 * time.Second},
		config: config,
		now: func() time.Time { return time.Now().UTC() },
	}
}

func (y *YandexOAuth) Enabled() bool {
	return strings.TrimSpace(y.config.ClientID) != "" && strings.TrimSpace(y.config.RedirectURL) != ""
}

func (y *YandexOAuth) WebCompleteURL() string { return y.config.WebCompleteURL }

func (y *YandexOAuth) Start(ctx context.Context, requestIP string) (string, error) {
	if !y.Enabled() {
		return "", fmt.Errorf("yandex oauth is not configured")
	}
	state, stateHash, err := newToken()
	if err != nil { return "", fmt.Errorf("generate oauth state: %w", err) }
	verifier, challenge, err := newPKCE()
	if err != nil { return "", fmt.Errorf("generate pkce: %w", err) }
	if err := y.store.CreateOAuthState(ctx, "yandex", stateHash, verifier, requestIP, y.now().Add(10*time.Minute)); err != nil {
		return "", fmt.Errorf("persist oauth state: %w", err)
	}

	query := url.Values{}
	query.Set("response_type", "code")
	query.Set("client_id", y.config.ClientID)
	query.Set("redirect_uri", y.config.RedirectURL)
	query.Set("state", state)
	query.Set("code_challenge", challenge)
	query.Set("code_challenge_method", "S256")
	return "https://oauth.yandex.ru/authorize?" + query.Encode(), nil
}

func (y *YandexOAuth) Complete(ctx context.Context, code, state, userAgent, requestIP string) (SessionTokens, error) {
	code = strings.TrimSpace(code)
	state = strings.TrimSpace(state)
	if code == "" || state == "" || len(code) > 2048 || len(state) > 1024 {
		return SessionTokens{}, ErrOAuthState
	}
	verifier, err := y.store.ConsumeOAuthState(ctx, "yandex", hashToken(state), y.now())
	if err != nil {
		if errors.Is(err, ErrOAuthState) { return SessionTokens{}, ErrOAuthState }
		return SessionTokens{}, fmt.Errorf("consume oauth state: %w", err)
	}

	oauthToken, err := y.exchangeCode(ctx, code, verifier)
	if err != nil { return SessionTokens{}, err }
	info, err := y.fetchUserInfo(ctx, oauthToken)
	if err != nil { return SessionTokens{}, err }
	if info.ID == "" || info.ClientID != y.config.ClientID {
		return SessionTokens{}, fmt.Errorf("yandex identity response mismatch")
	}

	userID, err := y.store.ResolveYandexIdentity(ctx, info.ID, strings.ToLower(strings.TrimSpace(info.DefaultEmail)), y.now())
	if err != nil { return SessionTokens{}, fmt.Errorf("resolve yandex identity: %w", err) }
	return y.issueSession(ctx, userID, userAgent, requestIP)
}

func (y *YandexOAuth) exchangeCode(ctx context.Context, code, verifier string) (string, error) {
	form := url.Values{}
	form.Set("grant_type", "authorization_code")
	form.Set("code", code)
	form.Set("client_id", y.config.ClientID)
	form.Set("code_verifier", verifier)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://oauth.yandex.ru/token", strings.NewReader(form.Encode()))
	if err != nil { return "", err }
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := y.client.Do(req)
	if err != nil { return "", fmt.Errorf("exchange yandex code: %w", err) }
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
	if err != nil { return "", err }
	if resp.StatusCode != http.StatusOK { return "", fmt.Errorf("yandex token exchange rejected") }
	var payload struct { AccessToken string `json:"access_token"` }
	if err := json.Unmarshal(body, &payload); err != nil || payload.AccessToken == "" {
		return "", fmt.Errorf("invalid yandex token response")
	}
	return payload.AccessToken, nil
}

type yandexUserInfo struct {
	ID           string `json:"id"`
	ClientID     string `json:"client_id"`
	DefaultEmail string `json:"default_email"`
}

func (y *YandexOAuth) fetchUserInfo(ctx context.Context, token string) (yandexUserInfo, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://login.yandex.ru/info?format=json", nil)
	if err != nil { return yandexUserInfo{}, err }
	req.Header.Set("Authorization", "OAuth "+token)
	resp, err := y.client.Do(req)
	if err != nil { return yandexUserInfo{}, fmt.Errorf("fetch yandex user info: %w", err) }
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK { return yandexUserInfo{}, fmt.Errorf("yandex user info rejected") }
	var info yandexUserInfo
	decoder := json.NewDecoder(io.LimitReader(resp.Body, 64<<10))
	if err := decoder.Decode(&info); err != nil { return yandexUserInfo{}, fmt.Errorf("decode yandex user info: %w", err) }
	return info, nil
}

func (y *YandexOAuth) issueSession(ctx context.Context, userID, userAgent, requestIP string) (SessionTokens, error) {
	now := y.now()
	accessToken, accessHash, err := newToken()
	if err != nil { return SessionTokens{}, err }
	refreshToken, refreshHash, err := newToken()
	if err != nil { return SessionTokens{}, err }
	tokens := SessionTokens{
		UserID: userID,
		AuthMethod: "yandex",
		AccessToken: accessToken,
		RefreshToken: refreshToken,
		AccessExpiry: now.Add(15 * time.Minute),
		RefreshExpiry: now.Add(30 * 24 * time.Hour),
	}
	sessionID, err := y.store.CreateSession(ctx, CreateSessionInput{
		UserID: userID,
		AuthMethod: "yandex",
		AccessTokenHash: accessHash,
		RefreshTokenHash: refreshHash,
		UserAgent: truncate(userAgent, 512),
		IP: requestIP,
		AccessExpiresAt: tokens.AccessExpiry,
		RefreshExpiresAt: tokens.RefreshExpiry,
	})
	if err != nil { return SessionTokens{}, fmt.Errorf("create yandex session: %w", err) }
	tokens.SessionID = sessionID
	return tokens, nil
}

func newPKCE() (string, string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil { return "", "", err }
	verifier := base64.RawURLEncoding.EncodeToString(buf)
	sum := sha256.Sum256([]byte(verifier))
	challenge := base64.RawURLEncoding.EncodeToString(sum[:])
	return verifier, challenge, nil
}
