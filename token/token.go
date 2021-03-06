package token

import (
	"context"
	"fmt"
	"io/ioutil"

	"github.com/fabric8-services/fabric8-tenant/auth"
	goaclient "github.com/goadesign/goa/client"
	"github.com/pkg/errors"
)

// Resolver resolves a Token for a given user/service
type Resolver func(ctx context.Context, target, token string, decode Decode) (user, accessToken string, err error)

// TenantResolver resolves tenant tokens based on tenants auth
type TenantResolver func(ctx context.Context, target, token string) (user, accessToken string, err error)

// NewAuthServiceResolver creates a Resolver that rely on the Auth service to retrieve tokens
func NewAuthServiceResolver(config AuthClientConfig) Resolver {
	c := tokenClient{config: config}
	return c.Get
}

type tokenClient struct {
	config AuthClientConfig
}

func (c *tokenClient) Get(ctx context.Context, target, token string, decode Decode) (string, string, error) {
	// auth can return empty token so validate against that
	if token == "" {
		return "", "", fmt.Errorf("access token can't be empty")
	}

	// check if the cluster is empty
	if target == "" {
		return "", "", fmt.Errorf("auth service returned an empty cluster url")
	}

	authclient, err := CreateClient(c.config)
	if err != nil {
		return "", "", err
	}
	authclient.SetJWTSigner(
		&goaclient.JWTSigner{
			TokenSource: &goaclient.StaticTokenSource{
				StaticToken: &goaclient.StaticToken{
					Value: token,
					Type:  "Bearer"}}})

	res, err := authclient.RetrieveToken(ctx, auth.RetrieveTokenPath(), target, nil)
	if err != nil {
		return "", "", errors.Wrapf(err, "error while doing the request")
	}
	defer func() {
		ioutil.ReadAll(res.Body)
		res.Body.Close()
	}()

	validationerror := validateError(authclient, res)
	if validationerror != nil {
		return "", "", errors.Wrapf(validationerror, "error from server %q", c.config.GetAuthURL())
	}

	externalToken, err := authclient.DecodeExternalToken(res)
	if err != nil {
		return "", "", errors.Wrapf(err, "error from server %q", c.config.GetAuthURL())
	}
	if externalToken.Username == nil {
		return "", "", errors.Wrapf(err, "missing username", c.config.GetAuthURL())
	}

	t, err := decode(externalToken.AccessToken)
	return *externalToken.Username, t, err
}
