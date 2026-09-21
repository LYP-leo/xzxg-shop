package domain

import "time"

type CartQuote struct {
	DiscountPreview
	AmountScope       string    `json:"amount_scope"`
	FulfillmentStatus string    `json:"fulfillment_status"`
	QuoteID           string    `json:"quote_id"`
	CartVersion       int64     `json:"cart_version"`
	Cart              Cart      `json:"cart"`
	Currency          string    `json:"currency"`
	PricingVersion    string    `json:"pricing_version"`
	ExpiresAt         time.Time `json:"expires_at"`
	// Returned only to the authenticated confirmation UI, never to the model.
	ConfirmationToken string `json:"confirmation_token,omitempty"`
}

type CheckoutConfirmation struct {
	QuoteID           string `json:"quote_id"`
	ConfirmationToken string `json:"confirmation_token"`
}
