package tekmetric

import "time"

// This file exposes internals to the external test package in this directory.
// The Go tool compiles it only during tests, so none of it ships in a binary.

// AccessToken returns the cached OAuth token.
func (c *Client) AccessToken() string { return c.accessToken }

// ShopIDs returns the shop list from the token scope.
func (c *Client) ShopIDs() []string { return c.shopIDs }

// TokenExpiry returns when the cached token expires.
func (c *Client) TokenExpiry() time.Time { return c.tokenExpiry }

// SetTokenExpiry overrides when the cached token expires, so a test can force a
// refresh without waiting.
func (c *Client) SetTokenExpiry(t time.Time) { c.tokenExpiry = t }

// IsAuthorizedShop reports whether the token scope covers a shop.
func (c *Client) IsAuthorizedShop(shopID int) error { return c.isAuthorizedShop(shopID) }
