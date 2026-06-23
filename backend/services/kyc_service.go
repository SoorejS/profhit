package services

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
)

// VerifyKYC hits a 3rd-party identity provider (e.g., Digilocker or HyperVerge).
// In a real app, this takes a base64 document or an Aadhaar/PAN number and fires an API request.
func VerifyKYC(documentID string) (bool, error) {
	kycEndpoint := os.Getenv("KYC_API_URL")
	kycApiKey := os.Getenv("KYC_API_KEY")

	// If no real API is configured, mock it.
	if kycEndpoint == "" {
		if documentID == "FAIL_KYC" {
			return false, nil
		}
		return true, nil // Mock success
	}

	payload, _ := json.Marshal(map[string]string{
		"document_id": documentID,
	})

	req, err := http.NewRequest("POST", kycEndpoint, bytes.NewBuffer(payload))
	if err != nil {
		return false, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+kycApiKey)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return false, fmt.Errorf("KYC failed with status: %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	var res map[string]interface{}
	json.Unmarshal(body, &res)

	// Assuming the API returns {"status": "verified"}
	if status, ok := res["status"].(string); ok && status == "verified" {
		return true, nil
	}

	return false, nil
}
