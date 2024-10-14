//go:build sync
// +build sync

package sync

import (
	"context"
	"encoding/base64"
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ecr"
	syncconf "zotregistry.dev/zot/pkg/extensions/config/sync"
	"zotregistry.dev/zot/pkg/log"
)

// ECR tokens are valid for 12 hours. The ExpiryWindow variable is set to 1 hour,
// meaning if the remaining validity of the token is less than 1 hour, it will be considered expired.
const ExpiryWindow int = 1

type ECRCredential struct {
	username string
	password string
	expiry   time.Time
	account  string
	region   string
}

type ECRCredentialsHelper struct {
	credentials map[string]ECRCredential
	log         log.Logger
}

func NewECRCredentialHelper(log log.Logger) CredentialHelper {
	return &ECRCredentialsHelper{
		credentials: make(map[string]ECRCredential),
		log:         log,
	}
}

// extractAccountAndRegion extracts the account ID and region from the given ECR URL
// Example URL format: account.dkr.ecr.region.amazonaws.com
func extractAccountAndRegion(url string) (string, string, error) {
	parts := strings.Split(url, ".")
	if len(parts) < 6 {
		return "", "", fmt.Errorf("invalid URL format: %s", url)
	}

	accountID := parts[0] // First part is the account ID
	region := parts[3]    // Fourth part is the region

	return accountID, region, nil
}

func GetECRCredentials(url string) (ECRCredential, error) {
	// Extract account ID and region from the URL
	accountID, region, err := extractAccountAndRegion(StripRegistryTransport(url))
	if err != nil {
		return ECRCredential{}, fmt.Errorf("failed to extract account and region from URL %s: %w", url, err)
	}

	// Load the AWS config for the specific region
	cfg, err := config.LoadDefaultConfig(context.TODO(), config.WithRegion(region))
	if err != nil {
		return ECRCredential{}, fmt.Errorf("unable to load AWS config for region %s: %w", region, err)
	}

	// Create an ECR client
	ecrClient := ecr.NewFromConfig(cfg)

	// Fetch the ECR authorization token
	ecrAuth, err := ecrClient.GetAuthorizationToken(context.TODO(), &ecr.GetAuthorizationTokenInput{
		RegistryIds: []string{accountID}, // Filter by the account ID
	})
	if err != nil {
		return ECRCredential{}, fmt.Errorf("unable to get ECR authorization token for account %s: %w", accountID, err)
	}

	// Decode the base64-encoded ECR token
	authToken := *ecrAuth.AuthorizationData[0].AuthorizationToken
	decodedToken, err := base64.StdEncoding.DecodeString(authToken)
	if err != nil {
		return ECRCredential{}, fmt.Errorf("unable to decode ECR token: %w", err)
	}

	// Split the decoded token into username and password (username is "AWS")
	tokenParts := strings.Split(string(decodedToken), ":")
	if len(tokenParts) != 2 {
		return ECRCredential{}, fmt.Errorf("invalid token format received from ECR")
	}

	expiry := *ecrAuth.AuthorizationData[0].ExpiresAt
	username := tokenParts[0]
	password := tokenParts[1]

	return ECRCredential{username: username, password: password, expiry: expiry, account: accountID, region: region}, nil

}

// GetECRCredentials retrieves the ECR credentials (username and password) from AWS ECR
func (credHelper *ECRCredentialsHelper) getCredentials(urls []string) (syncconf.CredentialsFile, error) {
	var ecrCredentials syncconf.CredentialsFile

	for _, url := range urls {
		ecrCred, err := GetECRCredentials(url)
		if err != nil {
			return syncconf.CredentialsFile{}, fmt.Errorf("failed to get ECR credentials for URL %s: %w", url, err)
		}
		// Store the credentials in the map using the base URL as the key
		ecrCredentials[url] = syncconf.Credentials{
			Username: ecrCred.username,
			Password: ecrCred.password,
		}
		credHelper.credentials[url] = ecrCred
	}

	return ecrCredentials, nil
}

func (credHelper *ECRCredentialsHelper) isCredentialsValid(url string) bool {
	expiry := credHelper.credentials[url].expiry
	expiryDuration := time.Duration(ExpiryWindow) * time.Hour
	if time.Until(expiry) <= expiryDuration {
		credHelper.log.Info().Str("url", url).Msg("The credentials are close to expiring")
		return false
	}

	credHelper.log.Debug().Str("url", url).Msg("The credentials are valid")
	return true
}

func (credHelper *ECRCredentialsHelper) refreshCredentials(url string) (syncconf.Credentials, error) {

	ecrCred, err := GetECRCredentials(url)
	if err != nil {
		return syncconf.Credentials{}, fmt.Errorf("failed to get ECR credentials for URL %s: %w", url, err)
	}
	return syncconf.Credentials{Username: ecrCred.username, Password: ecrCred.password}, nil

}
