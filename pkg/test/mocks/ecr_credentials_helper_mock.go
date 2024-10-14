// mock_ecr_credentials_helper.go
package mocks

import (
	"fmt"
	"strings"
	"time"

	"zotregistry.dev/zot/pkg/extensions/config/sync"
)

const ExpiryWindow int = 1

type ECRCredential struct {
	username string
	password string
	expiry   time.Time
	account  string
	region   string
}

// MockECRCredentialsHelper is a mock implementation of ECRCredentialsHelper
type MockECRCredentialsHelper struct {
	credentials map[string]ECRCredential
}

// NewMockECRCredentialsHelper creates a new instance of MockECRCredentialsHelper
func NewMockECRCredentialsHelper() *MockECRCredentialsHelper {
	return &MockECRCredentialsHelper{
		credentials: make(map[string]ECRCredential),
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

// Mock GetECRCredentials function
func (m *MockECRCredentialsHelper) GetECRCredentials(remoteAddress string) (ECRCredential, error) {
	// Simulate extracting account ID and region
	accountID, region, err := extractAccountAndRegion(remoteAddress)
	if err != nil {
		return ECRCredential{}, err
	}

	// Simulate returning mock credentials
	if accountID == "mockAccount" && region == "mockRegion" {
		return ECRCredential{
			username: "mockUsername",
			password: "mockPassword",
			expiry:   time.Now().Add(12 * time.Hour), // Set a valid expiry
			account:  accountID,
			region:   region,
		}, nil
	}

	return ECRCredential{}, fmt.Errorf("mock error for remote address: %s", remoteAddress)
}

// Mock method for getting credentials
func (m *MockECRCredentialsHelper) getCredentials(urls []string) (sync.CredentialsFile, error) {
	ecrCredentials := make(sync.CredentialsFile)

	for _, url := range urls {
		ecrCred, err := m.GetECRCredentials(url)
		if err != nil {
			return sync.CredentialsFile{}, err
		}

		ecrCredentials[url] = sync.Credentials{
			Username: ecrCred.username,
			Password: ecrCred.password,
		}
		m.credentials[url] = ecrCred
	}
	return ecrCredentials, nil
}

// Mock method for checking if credentials are valid
func (m *MockECRCredentialsHelper) isCredentialsValid(remoteAddress string) bool {
	if cred, exists := m.credentials[remoteAddress]; exists {
		return time.Until(cred.expiry) > time.Duration(ExpiryWindow)*time.Hour
	}
	return false
}

// Mock method for refreshing credentials
func (m *MockECRCredentialsHelper) refreshCredentials(remoteAddress string) (sync.Credentials, error) {
	ecrCred, err := m.GetECRCredentials(remoteAddress)
	if err != nil {
		return sync.Credentials{}, err
	}
	return sync.Credentials{Username: ecrCred.username, Password: ecrCred.password}, nil
}
