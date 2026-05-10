package ai

import "fmt"

type ErrorCode string

const (
	ErrorCodeProviderAuth  ErrorCode = "provider_auth"
	ErrorCodeProviderQuota ErrorCode = "provider_quota"
	ErrorCodeProviderRate  ErrorCode = "provider_rate_limit"
	ErrorCodeProviderError ErrorCode = "provider_error"
)

type ProviderError struct {
	Code       ErrorCode
	Provider   string
	StatusCode int
	Message    string
}

func (e ProviderError) Error() string {
	if e.StatusCode > 0 {
		return fmt.Sprintf("%s %s failed with status %d: %s", e.Provider, e.Code, e.StatusCode, e.Message)
	}
	return fmt.Sprintf("%s %s: %s", e.Provider, e.Code, e.Message)
}
