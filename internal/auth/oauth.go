package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"golang.org/x/oauth2"
	oauthGitHub "golang.org/x/oauth2/github"
	oauthGoogle "golang.org/x/oauth2/google"
)

// OAuthUserInfo holds the user profile retrieved from an OAuth provider.
type OAuthUserInfo struct {
	Provider   string
	ProviderID string
	Email      string
	Name       string
	AvatarURL  string
}

// OAuthProvider defines the interface for OAuth authentication providers.
type OAuthProvider interface {
	Name() string
	AuthCodeURL(state string) string
	Exchange(ctx context.Context, code string) (*oauth2.Token, error)
	GetUserInfo(ctx context.Context, token *oauth2.Token) (*OAuthUserInfo, error)
}

// ---------------------------------------------------------------------------
// Google
// ---------------------------------------------------------------------------

// GoogleProvider implements OAuthProvider for Google OAuth2.
type GoogleProvider struct {
	config *oauth2.Config
}

// NewGoogleProvider creates a Google OAuth provider with the given credentials.
func NewGoogleProvider(clientID, clientSecret, redirectURL string) *GoogleProvider {
	return &GoogleProvider{
		config: &oauth2.Config{
			ClientID:     clientID,
			ClientSecret: clientSecret,
			RedirectURL:  redirectURL,
			Scopes:       []string{"openid", "email", "profile"},
			Endpoint:     oauthGoogle.Endpoint,
		},
	}
}

func (g *GoogleProvider) Name() string { return "google" }

func (g *GoogleProvider) AuthCodeURL(state string) string {
	return g.config.AuthCodeURL(state, oauth2.AccessTypeOffline)
}

func (g *GoogleProvider) Exchange(ctx context.Context, code string) (*oauth2.Token, error) {
	token, err := g.config.Exchange(ctx, code)
	if err != nil {
		return nil, fmt.Errorf("google token exchange: %w", err)
	}
	return token, nil
}

// GetUserInfo fetches the authenticated user's profile from Google's userinfo endpoint.
func (g *GoogleProvider) GetUserInfo(ctx context.Context, token *oauth2.Token) (*OAuthUserInfo, error) {
	client := g.config.Client(ctx, token)
	resp, err := client.Get("https://www.googleapis.com/oauth2/v2/userinfo")
	if err != nil {
		return nil, fmt.Errorf("google userinfo request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("google userinfo returned %d: %s", resp.StatusCode, string(body))
	}

	var info struct {
		ID            string `json:"id"`
		Email         string `json:"email"`
		Name          string `json:"name"`
		Picture       string `json:"picture"`
		VerifiedEmail bool   `json:"verified_email"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return nil, fmt.Errorf("decoding google userinfo: %w", err)
	}

	return &OAuthUserInfo{
		Provider:   "google",
		ProviderID: info.ID,
		Email:      info.Email,
		Name:       info.Name,
		AvatarURL:  info.Picture,
	}, nil
}

// ---------------------------------------------------------------------------
// GitHub
// ---------------------------------------------------------------------------

// GithubProvider implements OAuthProvider for GitHub OAuth2.
type GithubProvider struct {
	config *oauth2.Config
}

// NewGithubProvider creates a GitHub OAuth provider with the given credentials.
func NewGithubProvider(clientID, clientSecret, redirectURL string) *GithubProvider {
	return &GithubProvider{
		config: &oauth2.Config{
			ClientID:     clientID,
			ClientSecret: clientSecret,
			RedirectURL:  redirectURL,
			Scopes:       []string{"user:email", "read:user"},
			Endpoint:     oauthGitHub.Endpoint,
		},
	}
}

func (g *GithubProvider) Name() string { return "github" }

func (g *GithubProvider) AuthCodeURL(state string) string {
	return g.config.AuthCodeURL(state)
}

func (g *GithubProvider) Exchange(ctx context.Context, code string) (*oauth2.Token, error) {
	token, err := g.config.Exchange(ctx, code)
	if err != nil {
		return nil, fmt.Errorf("github token exchange: %w", err)
	}
	return token, nil
}

// GetUserInfo fetches the authenticated user's profile from GitHub's API.
// If the user's profile email is empty, it falls back to the /user/emails endpoint.
func (g *GithubProvider) GetUserInfo(ctx context.Context, token *oauth2.Token) (*OAuthUserInfo, error) {
	client := g.config.Client(ctx, token)

	// Fetch primary profile.
	resp, err := client.Get("https://api.github.com/user")
	if err != nil {
		return nil, fmt.Errorf("github user request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("github user returned %d: %s", resp.StatusCode, string(body))
	}

	var user struct {
		ID        int64  `json:"id"`
		Login     string `json:"login"`
		Name      string `json:"name"`
		Email     string `json:"email"`
		AvatarURL string `json:"avatar_url"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&user); err != nil {
		return nil, fmt.Errorf("decoding github user: %w", err)
	}

	email := user.Email
	if email == "" {
		// GitHub may not return email in profile; fetch from /user/emails.
		var fetchedEmail string
		fetchedEmail, err = g.fetchPrimaryEmail(ctx, client)
		if err != nil {
			return nil, fmt.Errorf("fetching github email: %w", err)
		}
		email = fetchedEmail
	}

	name := user.Name
	if name == "" {
		name = user.Login
	}

	return &OAuthUserInfo{
		Provider:   "github",
		ProviderID: fmt.Sprintf("%d", user.ID),
		Email:      email,
		Name:       name,
		AvatarURL:  user.AvatarURL,
	}, nil
}

// fetchPrimaryEmail retrieves the user's primary verified email from GitHub.
func (g *GithubProvider) fetchPrimaryEmail(ctx context.Context, client *http.Client) (string, error) {
	resp, err := client.Get("https://api.github.com/user/emails")
	if err != nil {
		return "", fmt.Errorf("github emails request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("github emails returned %d: %s", resp.StatusCode, string(body))
	}

	var emails []struct {
		Email    string `json:"email"`
		Primary  bool   `json:"primary"`
		Verified bool   `json:"verified"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&emails); err != nil {
		return "", fmt.Errorf("decoding github emails: %w", err)
	}

	// Prefer primary + verified.
	for _, e := range emails {
		if e.Primary && e.Verified {
			return e.Email, nil
		}
	}
	// Fall back to any verified email.
	for _, e := range emails {
		if e.Verified {
			return e.Email, nil
		}
	}

	return "", fmt.Errorf("no verified email found on github account")
}
