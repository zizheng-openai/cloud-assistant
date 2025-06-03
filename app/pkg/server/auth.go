package server

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"net/url"
	"os"
	"slices"
	"strings"
	"sync"
	"time"

	connectcors "connectrpc.com/cors"
	"github.com/rs/cors"

	"github.com/jlewi/cloud-assistant/app/pkg/iam"
	"github.com/jlewi/cloud-assistant/app/pkg/logs"

	"github.com/go-logr/zapr"
	"github.com/golang-jwt/jwt/v5"
	"github.com/jlewi/cloud-assistant/app/pkg/config"
	"github.com/pkg/errors"
	"go.uber.org/zap"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

// IDTokenKeyType  string is the key used to store the ID token in the context
// We define a type to avoid collisions with other context keys
type IDTokenKeyType string

const (
	oidcPathPrefix                   = "/oidc"
	sessionCookieName                = "cassie-session"
	stateLength                      = 32
	IDTokenKey        IDTokenKeyType = "idToken"
)

// OIDC handles OAuth2 authentication setup and management
type OIDC struct {
	config     *config.OIDCConfig
	oauth2     *oauth2.Config
	publicKeys map[string]*rsa.PublicKey
	discovery  *openIDDiscovery
	state      *stateManager
	provider   OIDCProvider
}

// newOIDC creates a new OIDC
func newOIDC(cfg *config.OIDCConfig) (*OIDC, error) {
	if cfg == nil {
		return nil, nil
	}

	// Check that only one provider is configured
	if cfg.Google != nil && cfg.Generic != nil {
		return nil, errors.New("both Google and generic OIDC providers cannot be configured at the same time")
	}

	if cfg.Google == nil && cfg.Generic == nil {
		return nil, nil
	}

	var provider OIDCProvider
	if cfg.Google != nil {
		provider = NewGoogleProvider(cfg.Google)
	} else {
		provider = NewGenericProvider(cfg.Generic)
	}

	oauth2Config, err := provider.GetOAuth2Config()
	if err != nil {
		return nil, err
	}

	discoveryURL := provider.GetDiscoveryURL()

	// Fetch the OpenID configuration
	resp, err := http.Get(discoveryURL)
	if err != nil {
		return nil, errors.Wrap(err, "failed to fetch OpenID configuration")
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			zap.L().Error("failed to close response body", zap.Error(err))
		}
	}()

	var discovery openIDDiscovery
	if err := json.NewDecoder(resp.Body).Decode(&discovery); err != nil {
		return nil, errors.Wrap(err, "failed to decode OpenID configuration")
	}

	// Update endpoints from discovery document
	oauth2Config.Endpoint = oauth2.Endpoint{
		AuthURL:  discovery.AuthURL,
		TokenURL: discovery.TokenURL,
	}

	// If the generic provider is configured with an issuer, use it
	if cfg.Generic != nil && cfg.Generic.Issuer != "" {
		discovery.Issuer = cfg.Generic.Issuer
	}

	// Initialize OIDC
	oidc := &OIDC{
		config:     cfg,
		oauth2:     oauth2Config,
		publicKeys: make(map[string]*rsa.PublicKey),
		discovery:  &discovery,
		state:      newStateManager(10 * time.Minute),
		provider:   provider,
	}

	// Download JWKS for signature verification
	if err := oidc.downloadJWKS(); err != nil {
		return nil, errors.Wrapf(err, "Failed to download JWKS")
	}

	// Start a goroutine to clean up expired states
	go func() {
		ticker := time.NewTicker(oidc.state.stateExpiration / 2)
		defer ticker.Stop()
		for range ticker.C {
			oidc.state.cleanupExpiredStates()
		}
	}()

	return oidc, nil
}

// downloadJWKS downloads the JSON Web Key Set (JWKS) from Google's OAuth2 provider.
// It fetches the public keys used to verify JWT signatures, decodes them from the
// JWK format, and stores them in the OIDC instance's publicKeys map indexed by key ID.
// This allows the application to verify tokens offline without contacting Google's servers
// for each verification request.
func (o *OIDC) downloadJWKS() error {
	// Fetch the JWKS from the URI specified in the discovery document
	resp, err := http.Get(o.discovery.JWKSURI)
	if err != nil {
		return errors.Wrapf(err, "Failed to fetch JWKS from %s", o.discovery.JWKSURI)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			zap.L().Error("failed to close response body", zap.Error(err))
		}
	}()

	// Parse the JWKS into our structured format
	var jwks jwks
	if err := json.NewDecoder(resp.Body).Decode(&jwks); err != nil {
		return errors.Wrapf(err, "Failed to parse JWKS response")
	}

	// Convert each key to RSA public key and store in the map
	for _, key := range jwks.Keys {
		// Convert the modulus and exponent from base64url to *rsa.PublicKey
		n, err := base64.RawURLEncoding.DecodeString(key.N)
		if err != nil {
			return errors.Wrap(err, "failed to decode modulus")
		}

		e, err := base64.RawURLEncoding.DecodeString(key.E)
		if err != nil {
			return errors.Wrap(err, "failed to decode exponent")
		}

		// Convert the modulus to a big integer
		modulus := new(big.Int).SetBytes(n)

		// Convert the exponent to an integer
		var exponent int
		if len(e) < 4 {
			for i := range e {
				exponent = exponent<<8 + int(e[i])
			}
		} else {
			return errors.New("exponent too large")
		}

		// Create the RSA public key
		publicKey := &rsa.PublicKey{
			N: modulus,
			E: exponent,
		}

		// Store the public key in the map using the kid as the key
		o.publicKeys[key.Kid] = publicKey
	}

	return nil
}

// verifyBearerToken verifies the JWT token in the bearer token and returns whether it's valid and any error encountered
func (o *OIDC) verifyBearerToken(bearerToken string) (*jwt.Token, error) {
	if !strings.HasPrefix(strings.ToLower(bearerToken), "bearer ") {
		return nil, errors.New("bearer token must start with 'Bearer '")
	}

	// Remove prefix case-insensitive, trim whitespace
	idToken := strings.TrimPrefix(bearerToken, bearerToken[:7])
	idToken = strings.TrimSpace(idToken)

	return o.verifyToken(idToken)
}

// verifyToken verifies the JWT token and returns whether it's valid and any error encountered
// result is nil if its an invalid token and non-nil if its valid
func (o *OIDC) verifyToken(idToken string) (*jwt.Token, error) {
	if o == nil {
		return nil, errors.New("OIDC is not configured")
	}

	// Verify the token signature using JWKS
	token, err := jwt.Parse(idToken, func(token *jwt.Token) (any, error) {
		// Verify the signing method is what we expect
		if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}

		// Get the key ID from the token header
		kid, ok := token.Header["kid"].(string)
		if !ok {
			return nil, errors.New("kid header not found in token")
		}

		// Get the public key from our map
		publicKey, ok := o.publicKeys[kid]
		if !ok {
			return nil, errors.New("unable to find appropriate key")
		}

		return publicKey, nil
	})

	if err != nil || !token.Valid {
		return nil, fmt.Errorf("invalid token signature: %v", err)
	}

	// Get the claims from the token
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return nil, errors.New("failed to get claims from token")
	}

	// Verify expiration
	exp, err := claims.GetExpirationTime()
	if err != nil {
		return nil, fmt.Errorf("failed to get expiration from claims: %v", err)
	}
	if time.Now().After(exp.Time) {
		return nil, errors.New("token expired")
	}

	// Verify issuer
	iss, err := claims.GetIssuer()
	if err != nil || iss != o.discovery.Issuer {
		return nil, fmt.Errorf("invalid token issuer: got %v, expected %v", iss, o.discovery.Issuer)
	}

	// Verify audience matches our client ID
	aud, err := claims.GetAudience()
	if err != nil || len(aud) == 0 || aud[0] != o.oauth2.ClientID {
		return nil, fmt.Errorf("invalid token audience: got %v, expected %v", aud, o.oauth2.ClientID)
	}

	return token, nil
}

// RegisterAuthRoutes registers the OAuth2 authentication routes
func RegisterAuthRoutes(oidc *OIDC, mux *AuthMux) error {
	if oidc == nil {
		return nil
	}

	// Register OAuth2 endpoints
	mux.HandleFunc(oidcPathPrefix+"/login", oidc.loginHandler)
	mux.HandleFunc(oidcPathPrefix+"/callback", oidc.callbackHandler)
	mux.HandleFunc(oidcPathPrefix+"/logout", oidc.logoutHandler)
	mux.HandleFunc("/logout", oidc.logoutHandler)

	return nil
}

// NewAuthMiddleware creates a middleware that enforces OIDC authentication
func NewAuthMiddleware(oidc *OIDC) (func(http.Handler) http.Handler, error) {
	return newAuthMiddlewareForOIDC(oidc)
}

// newAuthMiddlewareForOIDC allows our test to inject a mock OIDC
func newAuthMiddlewareForOIDC(oidc *OIDC) (func(http.Handler) http.Handler, error) {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			log := zapr.NewLogger(zap.L())

			// Skip authentication for login page and OAuth2 endpoints
			if r.URL.Path == "/login" || strings.HasPrefix(r.URL.Path, oidcPathPrefix+"/") {
				next.ServeHTTP(w, r)
				return
			}

			var token *jwt.Token
			var err error

			// Prefer bearer token over session cookie
			bearerToken := r.Header.Get("Authorization")
			if strings.HasPrefix(strings.ToLower(bearerToken), "bearer ") {
				token, err = oidc.verifyBearerToken(bearerToken)
			} else {
				// Fallback to session cookie
				cookie, cookieErr := r.Cookie(sessionCookieName)
				if cookieErr != nil {
					// No session cookie or bearer token, return 401 Unauthorized
					log.Error(cookieErr, "No session cookie or bearer token found")
					http.Error(w, "Unauthorized: No valid session", http.StatusUnauthorized)
					return
				}
				token, err = oidc.verifyToken(cookie.Value)
			}

			if token == nil {
				log.Error(err, "Token validation failed")
				// Return HTTP 401 and let the client handle the redirect to the login page
				http.Error(w, "Unauthorized: Invalid or expired token", http.StatusUnauthorized)
				return
			}

			ctx := contextWithIDToken(r.Context(), token)

			// Token is valid, proceed with the request
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}, nil
}

// loginHandler handles the OAuth2 login flow
func (o *OIDC) loginHandler(w http.ResponseWriter, r *http.Request) {
	state, err := o.state.generateState()
	if err != nil {
		log := zapr.NewLogger(zap.L())
		log.Error(err, "Failed to generate state")
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	url := o.oauth2.AuthCodeURL(state)
	if o.config.ForceApproval {
		url = o.oauth2.AuthCodeURL(state, oauth2.ApprovalForce)
	}
	http.Redirect(w, r, url, http.StatusFound)
}

// callbackHandler handles the OAuth2 callback
func (o *OIDC) callbackHandler(w http.ResponseWriter, r *http.Request) {
	log := zapr.NewLogger(zap.L())

	// Verify state
	state := r.URL.Query().Get("state")
	if !o.state.validateState(state) {
		log.Error(nil, "Invalid state parameter")
		redirectWithError(w, r, "invalid_state", "Invalid state parameter")
		return
	}

	// Exchange code for token
	code := r.URL.Query().Get("code")
	token, err := o.oauth2.Exchange(r.Context(), code)
	if err != nil {
		log.Error(err, "Failed to exchange code for token")
		redirectWithError(w, r, "token_exchange_failed", "Failed to exchange code for token")
		return
	}

	// Get the ID token from the response
	idToken, ok := token.Extra("id_token").(string)
	if !ok {
		log.Error(nil, "No ID token in response")
		redirectWithError(w, r, "no_id_token", "No ID token in response")
		return
	}

	// Set the session cookie with the ID token
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    idToken,
		Path:     "/",
		HttpOnly: false,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
	})

	// Redirect to the home page
	http.Redirect(w, r, "/", http.StatusFound)
}

// redirectWithError redirects to the login page with error information
func redirectWithError(w http.ResponseWriter, r *http.Request, errorCode, errorDescription string) {
	// Get any existing error parameters from the request
	existingError := r.URL.Query().Get("error")
	existingDescription := r.URL.Query().Get("error_description")

	// Use existing error parameters if they exist, otherwise use the provided ones
	if existingError == "" {
		existingError = errorCode
	}
	if existingDescription == "" {
		existingDescription = errorDescription
	}

	// Build the redirect URL with error parameters
	redirectURL := fmt.Sprintf("/login?error=%s&error_description=%s",
		url.QueryEscape(existingError),
		url.QueryEscape(existingDescription))

	http.Redirect(w, r, redirectURL, http.StatusFound)
}

// logoutHandler handles the OAuth2 logout
func (o *OIDC) logoutHandler(w http.ResponseWriter, r *http.Request) {
	// Clear the session cookie
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	})

	// Redirect to the home page
	http.Redirect(w, r, "/", http.StatusFound)
}

type stateEntry struct {
	expiresAt time.Time
}

type stateManager struct {
	stateExpiration time.Duration
	states          map[string]stateEntry
	mu              sync.RWMutex
}

func newStateManager(stateExpiration time.Duration) *stateManager {
	return &stateManager{
		stateExpiration: stateExpiration,
		states:          make(map[string]stateEntry),
	}
}

// generateState generates a new cryptographically secure random state
func (sm *stateManager) generateState() (string, error) {
	b := make([]byte, stateLength)
	if _, err := rand.Read(b); err != nil {
		return "", errors.Wrap(err, "failed to generate random state")
	}
	state := base64.URLEncoding.EncodeToString(b)

	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.states[state] = stateEntry{
		expiresAt: time.Now().Add(sm.stateExpiration),
	}

	return state, nil
}

// validateState checks if a state is valid and removes it if it is
func (sm *stateManager) validateState(state string) bool {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	entry, exists := sm.states[state]
	if !exists {
		return false
	}

	// Remove the state regardless of validity
	delete(sm.states, state)

	// Check if the state has expired
	return time.Now().Before(entry.expiresAt)
}

// cleanupExpiredStates removes expired states from the map
func (sm *stateManager) cleanupExpiredStates() {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	now := time.Now()
	for state, entry := range sm.states {
		if now.After(entry.expiresAt) {
			delete(sm.states, state)
		}
	}
}

// jwksKey represents a single key in the JWKS
type jwksKey struct {
	Kty string `json:"kty"`
	Alg string `json:"alg"`
	Use string `json:"use"`
	Kid string `json:"kid"`
	N   string `json:"n"`
	E   string `json:"e"`
}

// jwks represents the JSON Web Key Set
type jwks struct {
	Keys []jwksKey `json:"keys"`
}

// openIDDiscovery is the struct for parsing the discovery document
type openIDDiscovery struct {
	Issuer                           string   `json:"issuer"`
	AuthURL                          string   `json:"authorization_endpoint"`
	TokenURL                         string   `json:"token_endpoint"`
	JWKSURI                          string   `json:"jwks_uri"`
	UserInfoURL                      string   `json:"userinfo_endpoint"`
	ScopesSupported                  []string `json:"scopes_supported"`
	ResponseTypesSupported           []string `json:"response_types_supported"`
	SubjectTypesSupported            []string `json:"subject_types_supported"`
	IDTokenSigningAlgValuesSupported []string `json:"id_token_signing_alg_values_supported"`
}

// OIDCProvider defines the interface for OIDC providers
type OIDCProvider interface {
	// GetOAuth2Config returns the OAuth2 configuration
	GetOAuth2Config() (*oauth2.Config, error)
	// GetDiscoveryURL returns the OpenID Connect discovery URL
	GetDiscoveryURL() string
}

// GoogleProvider implements OIDCProvider for Google OAuth2
type GoogleProvider struct {
	config *config.GoogleOIDCConfig
}

func NewGoogleProvider(cfg *config.GoogleOIDCConfig) *GoogleProvider {
	return &GoogleProvider{config: cfg}
}

func (p *GoogleProvider) GetOAuth2Config() (*oauth2.Config, error) {
	bytes, err := os.ReadFile(p.config.ClientCredentialsFile)
	if err != nil {
		return nil, errors.Wrapf(err, "Failed to read client credentials file")
	}

	config, err := google.ConfigFromJSON(bytes, "openid", "email")
	if err != nil {
		return nil, errors.Wrapf(err, "Failed to create OAuth2 config")
	}
	return config, nil
}

func (p *GoogleProvider) GetDiscoveryURL() string {
	return p.config.GetDiscoveryURL()
}

func (p *GoogleProvider) ValidateDomainClaims(claims jwt.MapClaims, allowedDomains []string) error {
	hd, ok := claims["hd"].(string)
	if !ok {
		return errors.New("missing hosted domain claim")
	}
	if hd != "" {
		if !slices.Contains(allowedDomains, hd) {
			return fmt.Errorf("hosted domain %v not in allowed domains", hd)
		}
	} else {
		return errors.New("missing hosted domain claim")
	}
	return nil
}

// GenericProvider implements OIDCProvider for generic OIDC providers
type GenericProvider struct {
	config *config.GenericOIDCConfig
}

func NewGenericProvider(cfg *config.GenericOIDCConfig) *GenericProvider {
	return &GenericProvider{config: cfg}
}

func (p *GenericProvider) GetOAuth2Config() (*oauth2.Config, error) {
	return &oauth2.Config{
		ClientID:     p.config.ClientID,
		ClientSecret: p.config.ClientSecret,
		RedirectURL:  p.config.RedirectURL,
		Scopes:       p.config.Scopes,
	}, nil
}

func (p *GenericProvider) GetDiscoveryURL() string {
	return p.config.GetDiscoveryURL()
}

func (p *GenericProvider) ValidateDomainClaims(claims jwt.MapClaims, allowedDomains []string) error {
	email, ok := claims["email"].(string)
	if !ok {
		return errors.New("missing email claim")
	}

	if email == "" {
		return errors.New("empty email claim")
	}

	parts := strings.Split(email, "@")
	if len(parts) != 2 {
		return errors.New("invalid email format")
	}

	emailDomain := parts[1]
	if !slices.Contains(allowedDomains, emailDomain) {
		return fmt.Errorf("email domain not in allowed domains: %s", email)
	}

	return nil
}

// AuthMux wraps http.ServeMux to add protected route handling
type AuthMux struct {
	mux            *http.ServeMux
	authMiddleware func(http.Handler) http.Handler
	serverConfig   *config.AssistantServerConfig
}

// NewAuthMux creates a new AuthMux
func NewAuthMux(serverConfig *config.AssistantServerConfig, oidc *OIDC) (*AuthMux, error) {
	mux := http.NewServeMux()

	// Create auth middleware if OIDC is configured
	var authMiddleware func(http.Handler) http.Handler
	if oidc != nil {
		middleware, err := NewAuthMiddleware(oidc)
		if err != nil {
			return nil, errors.Wrapf(err, "Failed to create auth middleware")
		}
		authMiddleware = middleware
	} else {
		// No-op middleware if OIDC is not configured
		authMiddleware = func(next http.Handler) http.Handler {
			return next
		}
	}

	return &AuthMux{
		mux:            mux,
		authMiddleware: authMiddleware,
		serverConfig:   serverConfig,
	}, nil
}

// Handle registers a handler for the given pattern
func (p *AuthMux) Handle(pattern string, handler http.Handler) {
	p.mux.Handle(pattern, handler)
}

// HandleFunc registers a handler function for the given pattern
func (p *AuthMux) HandleFunc(pattern string, handler func(http.ResponseWriter, *http.Request)) {
	p.mux.HandleFunc(pattern, handler)
}

// HandleProtected registers a protected handler for the given pattern
func (p *AuthMux) HandleProtected(pattern string, handler http.Handler, checker iam.Checker, role string) {
	// We need to create a new Auth middleware that will apply the IAM checks specific to this handler
	iamChecker := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

			log := logs.FromContext(r.Context())

			idToken, err := getIDToken(r.Context())
			// Nil token is not fatal until authz check
			if err != nil {
				log.Info("Unauthenticated: ", "error", err)
			}

			principal, err := checker.GetPrincipal(idToken)
			if err != nil {
				log.Info("Unauthorized: ", "error", err)
				http.Error(w, "Unauthorized: "+err.Error(), http.StatusUnauthorized)
				return
			}

			if !checker.Check(principal, role) {
				log.Info("Unauthorized", "user", principal, "role", role)
				http.Error(w, fmt.Sprintf("Forbidden: user %s doesn't have role %s", principal, role), http.StatusForbidden)
				return
			}

			// Get the IDToken from the context
			next.ServeHTTP(w, r)
		})
	}
	log := logs.NewLogger()
	// Create a chain: cors > check the IDToken is valid -> Apply AuthZ -> call the handler
	// CORS needs to come first because it will terminate the request chain on OPTIONS requests
	// OPTIONS requests won't carry authorization headers so we can't do authorization first
	handler = p.authMiddleware(iamChecker(handler))
	// Apply CORS if origins are configured
	// This is modeled on cors.AllowAll() but we can't use that because we need to allow credentials
	corsOptions := cors.Options{
		AllowedOrigins: p.serverConfig.CorsOrigins,
		// Allow all methods.
		AllowedMethods: []string{
			http.MethodHead,
			http.MethodGet,
			http.MethodPost,
			http.MethodPut,
			http.MethodPatch,
			http.MethodDelete,
		},
		// Allow all headers.
		AllowedHeaders:   []string{"*"},
		ExposedHeaders:   connectcors.ExposedHeaders(),
		AllowCredentials: true,
		MaxAge:           7200, // 2 hours in seconds
	}
	if len(p.serverConfig.CorsOrigins) > 0 {
		log.Info("Adding CORS support", "AllowedOrigins", corsOptions.AllowedOrigins, "AllowCredentials", corsOptions.AllowCredentials, "AllowedMethods", corsOptions.AllowedMethods, "AllowedHeaders", corsOptions.AllowedHeaders, "ExposedHeaders", corsOptions.ExposedHeaders)

		if p.serverConfig.CorsOrigins[0] == "*" {
			log.Info("Allowing all origins; enabling SetOriginHeader middleware")
			// We need to set the origin header to the request's origin
			// To do that we need to set the passthrough option to true so that the handler will invoke our middleware
			// after calling the cors handler
			corsOptions.OptionsPassthrough = true
			corsOptions.Debug = true
			c := cors.New(corsOptions)
			handler = c.Handler(SetOriginHeader(handler))
		} else {
			c := cors.New(corsOptions)
			handler = c.Handler(handler)
		}

	}

	p.mux.Handle(pattern, handler)
}

// HandleProtectedFunc registers a protected handler function for the given pattern
func (p *AuthMux) HandleProtectedFunc(pattern string, handler func(http.ResponseWriter, *http.Request)) {
	p.mux.Handle(pattern, p.authMiddleware(http.HandlerFunc(handler)))
}

// ServeHTTP implements http.Handler
func (p *AuthMux) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	p.mux.ServeHTTP(w, r)
}
