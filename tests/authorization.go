package tests

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

type authorizationCodeTokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	IDToken      string `json:"id_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
	Scope        string `json:"scope"`
}

func buildAuthorizationURL(
	authServerURL string,
	realm string,
	clientID string,
	redirectURL string,
	state string,
	codeChallenge string,
) string {
	endpoint := fmt.Sprintf(
		"%s/realms/%s/protocol/openid-connect/auth",
		strings.TrimRight(authServerURL, "/"),
		url.PathEscape(realm),
	)

	query := url.Values{}
	query.Set("client_id", clientID)
	query.Set("redirect_uri", redirectURL)
	query.Set("response_type", "code")
	query.Set("scope", "openid profile email")
	query.Set("state", state)
	query.Set("code_challenge", codeChallenge)
	query.Set("code_challenge_method", "S256")

	return endpoint + "?" + query.Encode()
}

func exchangeAuthorizationCode(
	ctx context.Context,
	authServerURL string,
	realm string,
	clientID string,
	redirectURL string,
	code string,
	codeVerifier string,
) (*authorizationCodeTokenResponse, error) {
	tokenURL := fmt.Sprintf(
		"%s/realms/%s/protocol/openid-connect/token",
		strings.TrimRight(authServerURL, "/"),
		url.PathEscape(realm),
	)

	form := url.Values{}
	form.Set("grant_type", "authorization_code")
	form.Set("client_id", clientID)
	form.Set("redirect_uri", redirectURL)
	form.Set("code", code)
	form.Set("code_verifier", codeVerifier)

	request, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		tokenURL,
		strings.NewReader(form.Encode()),
	)
	if err != nil {
		return nil, fmt.Errorf("create token request: %w", err)
	}

	request.Header.Set(
		"Content-Type",
		"application/x-www-form-urlencoded",
	)

	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return nil, fmt.Errorf("send token request: %w", err)
	}
	defer response.Body.Close()

	body, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, fmt.Errorf("read token response: %w", err)
	}

	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf(
			"token exchange returned %s: %s",
			response.Status,
			string(body),
		)
	}

	var tokens authorizationCodeTokenResponse

	if err := json.Unmarshal(body, &tokens); err != nil {
		return nil, fmt.Errorf("decode token response: %w", err)
	}

	return &tokens, nil
}
