// Package api defines the structures for the infection publishing API.
package api

// Publish represents the body of the PublishInfectedIds API call.
type Publish struct {
	Keys           []string `json:"diagnosisKeys"`
	AppPackageName string   `json:"appPackageName"`
	Country        string   `json:"country"`
	Platform       string   `json:"platform"`
	Verification   string   `json:"verificationPayload"`
}
