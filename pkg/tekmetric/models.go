package tekmetric

import (
	"encoding/json"
)

// Currency represents a monetary value in cents that outputs as dollars
type Currency int

// MarshalJSON formats Currency as dollars (dividing cents by 100)
func (c Currency) MarshalJSON() ([]byte, error) {
	dollars := float64(c) / 100.0
	return json.Marshal(dollars)
}

// UnmarshalJSON parses Currency from cents
func (c *Currency) UnmarshalJSON(data []byte) error {
	var cents int
	if err := json.Unmarshal(data, &cents); err != nil {
		return err
	}
	*c = Currency(cents)
	return nil
}

// TokenResponse represents the OAuth token response
type TokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	ExpiresIn   int    `json:"expires_in"` // Token lifetime in seconds
	Scope       string `json:"scope"`      // Space-separated shop IDs
}

// Address represents a physical address
type Address struct {
	ID            int    `json:"id"`
	Address1      string `json:"address1"`
	Address2      string `json:"address2"`
	City          string `json:"city"`
	State         string `json:"state"`
	Zip           string `json:"zip"`
	StreetAddress string `json:"streetAddress"`
	FullAddress   string `json:"fullAddress"`
}

// Phone represents a phone number
type Phone struct {
	ID      int    `json:"id,omitempty"`
	Number  string `json:"number"`
	Type    string `json:"type"`
	Primary bool   `json:"primary"`
}

// PaginatedResponse represents a paginated API response
type PaginatedResponse[T any] struct {
	Content          []T  `json:"content"`
	TotalPages       int  `json:"totalPages"`
	TotalElements    int  `json:"totalElements"`
	Last             bool `json:"last"`
	First            bool `json:"first"`
	Size             int  `json:"size"`
	Number           int  `json:"number"`
	NumberOfElements int  `json:"numberOfElements"`
	Empty            bool `json:"empty"`
}
