package mintarex

// Environment distinguishes live from sandbox mode.
type Environment string

const (
	EnvLive    Environment = "live"
	EnvSandbox Environment = "sandbox"
)

// CurrencyType classifies a currency as fiat or crypto.
type CurrencyType string

const (
	FiatCurrency   CurrencyType = "fiat"
	CryptoCurrency CurrencyType = "crypto"
)

// Pagination metadata returned by list endpoints.
type Pagination struct {
	Total   int  `json:"total"`
	Limit   int  `json:"limit"`
	Offset  int  `json:"offset"`
	HasMore bool `json:"has_more"`
}

// Balance is one row in a BalancesResponse.
type Balance struct {
	Currency     string       `json:"currency"`
	CurrencyType CurrencyType `json:"currency_type"`
	Available    string       `json:"available"`
	Locked       string       `json:"locked"`
	PendingIn    string       `json:"pending_in"`
	PendingOut   string       `json:"pending_out"`
	Total        string       `json:"total"`
	USDValue     *string      `json:"usd_value,omitempty"`
	USDPrice     *string      `json:"usd_price,omitempty"`
}

// BalancesResponse is returned by [AccountResource.Balances].
type BalancesResponse struct {
	Balances  []Balance `json:"balances"`
	Timestamp string    `json:"timestamp"`
	Meta      *ResponseMeta
}

// WalletTypeBalance is a per-wallet-type entry inside SingleBalanceResponse.
type WalletTypeBalance struct {
	WalletType string `json:"wallet_type"`
	Available  string `json:"available"`
	Locked     string `json:"locked"`
	PendingIn  string `json:"pending_in"`
	PendingOut string `json:"pending_out"`
}

// SingleBalanceResponse is returned by [AccountResource.Balance].
type SingleBalanceResponse struct {
	Currency        string              `json:"currency"`
	CurrencyType    CurrencyType        `json:"currency_type"`
	TotalAvailable  string              `json:"total_available"`
	TotalLocked     string              `json:"total_locked"`
	TotalPendingIn  string              `json:"total_pending_in"`
	TotalPendingOut string              `json:"total_pending_out"`
	Total           string              `json:"total"`
	ByWalletType    []WalletTypeBalance `json:"by_wallet_type"`
	Timestamp       string              `json:"timestamp"`
	Meta            *ResponseMeta
}

// LimitBucket is one category of limit returned in LimitsResponse. The
// *_used and remaining_* fields may be nil when the server has not computed
// them yet.
type LimitBucket struct {
	DailyLimit       *string `json:"daily_limit"`
	DailyUsed        *string `json:"daily_used"`
	MonthlyLimit     *string `json:"monthly_limit"`
	MonthlyUsed      *string `json:"monthly_used"`
	RemainingDaily   *string `json:"remaining_daily"`
	RemainingMonthly *string `json:"remaining_monthly"`
}

// LimitsResponse is returned by [AccountResource.Limits]. Only crypto
// deposit/withdrawal limits are exposed; fiat operations (bank, card,
// e-wallet) are dashboard-only and not returned.
type LimitsResponse struct {
	AccountType string `json:"account_type"` // "individual" | "corporate"
	Limits      struct {
		CryptoDeposit    *LimitBucket `json:"crypto_deposit"`
		CryptoWithdrawal *LimitBucket `json:"crypto_withdrawal"`
	} `json:"limits"`
	Timestamp string `json:"timestamp"`
	Meta      *ResponseMeta
}

// QuoteRequest is the input to [RFQResource.Quote]. Required fields are Base,
// Quote, Side, Amount, AmountType. Network / FromNetwork / ToNetwork are used
// for crypto-crypto swaps where applicable.
type QuoteRequest struct {
	Base        string `json:"base"`
	Quote       string `json:"quote"`
	Side        string `json:"side"` // "buy" | "sell"
	Amount      string `json:"amount"`
	AmountType  string `json:"amount_type"` // "base" | "quote"
	Network     string `json:"network,omitempty"`
	FromNetwork string `json:"from_network,omitempty"`
	ToNetwork   string `json:"to_network,omitempty"`
}

// Quote is returned by [RFQResource.Quote].
type Quote struct {
	QuoteID     string `json:"quote_id"`
	Base        string `json:"base"`
	Quote       string `json:"quote"`
	Side        string `json:"side"`
	Network     string `json:"network"`
	Price       string `json:"price"`
	BaseAmount  string `json:"base_amount"`
	QuoteAmount string `json:"quote_amount"`
	ExpiresAt   string `json:"expires_at"`
	ExpiresInMs int64  `json:"expires_in_ms"`
	Meta        *ResponseMeta
}

// TradeExecution is returned by [RFQResource.Accept].
type TradeExecution struct {
	TradeID     string `json:"trade_id"`
	Status      string `json:"status"` // "filled" | "pending" | "cancelled" | "failed" | "expired"
	Base        string `json:"base"`
	Quote       string `json:"quote"`
	Side        string `json:"side"`
	Network     string `json:"network"`
	Price       string `json:"price"`
	BaseAmount  string `json:"base_amount"`
	QuoteAmount string `json:"quote_amount"`
	FilledAt    string `json:"filled_at"`
	IsSwap      bool   `json:"is_swap,omitempty"`
	FromNetwork string `json:"from_network,omitempty"`
	ToNetwork   string `json:"to_network,omitempty"`
	Sandbox     bool   `json:"sandbox,omitempty"`
	Idempotent  bool   `json:"idempotent,omitempty"`
	Meta        *ResponseMeta
}

// Trade is returned by [TradesResource.Get] and [TradesResource.List].
type Trade struct {
	TradeID     string `json:"trade_id"`
	Base        string `json:"base"`
	Quote       string `json:"quote"`
	Side        string `json:"side"`
	Status      string `json:"status"`
	Price       string `json:"price"`
	BaseAmount  string `json:"base_amount"`
	QuoteAmount string `json:"quote_amount"`
	FeeAmount   string `json:"fee_amount"`
	FeeCurrency string `json:"fee_currency"`
	OrderType   string `json:"order_type"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at,omitempty"`
	Sandbox     bool   `json:"sandbox,omitempty"`
	Meta        *ResponseMeta
}

// TradesList is the shape returned by [TradesResource.List].
type TradesList struct {
	Data       []Trade    `json:"data"`
	Pagination Pagination `json:"pagination"`
	Meta       *ResponseMeta
}

// DepositAddress is returned by [CryptoResource.DepositAddress].
type DepositAddress struct {
	Address               string `json:"address"`
	Coin                  string `json:"coin"`
	Network               string `json:"network"`
	MemoRequired          bool   `json:"memo_required"`
	MinDeposit            string `json:"min_deposit"`
	RequiredConfirmations int    `json:"required_confirmations"`
	Timestamp             string `json:"timestamp"`
	Meta                  *ResponseMeta
}

// CryptoDeposit is one row in [CryptoDepositsList].
type CryptoDeposit struct {
	DepositID             string  `json:"deposit_id"`
	Coin                  string  `json:"coin"`
	Network               string  `json:"network"`
	Amount                string  `json:"amount"`
	TxHash                string  `json:"tx_hash"`
	FromAddress           *string `json:"from_address"`
	Confirmations         int     `json:"confirmations"`
	RequiredConfirmations int     `json:"required_confirmations"`
	Status                string  `json:"status"`
	DetectedAt            string  `json:"detected_at"`
	UpdatedAt             string  `json:"updated_at"`
	Sandbox               bool    `json:"sandbox,omitempty"`
}

// CryptoDepositsList is returned by [CryptoResource.Deposits].
type CryptoDepositsList struct {
	Data       []CryptoDeposit `json:"data"`
	Pagination Pagination      `json:"pagination"`
	Meta       *ResponseMeta
}

// CryptoWithdrawal is returned by withdrawal endpoints.
type CryptoWithdrawal struct {
	WithdrawalID  string  `json:"withdrawal_id"`
	Reference     *string `json:"reference,omitempty"`
	Coin          string  `json:"coin"`
	Network       string  `json:"network"`
	Amount        string  `json:"amount"`
	Fee           string  `json:"fee"`
	TotalDeducted string  `json:"total_deducted,omitempty"`
	AmountUSD     *string `json:"amount_usd,omitempty"`
	ToAddress     string  `json:"to_address"`
	Memo          *string `json:"memo,omitempty"`
	TxHash        *string `json:"tx_hash,omitempty"`
	ExplorerURL   *string `json:"explorer_url,omitempty"`
	Status        string  `json:"status"`
	RejectReason  *string `json:"reject_reason,omitempty"`
	ReviewedAt    *string `json:"reviewed_at,omitempty"`
	BroadcastAt   *string `json:"broadcast_at,omitempty"`
	CompletedAt   *string `json:"completed_at,omitempty"`
	CreatedAt     string  `json:"created_at,omitempty"`
	UpdatedAt     string  `json:"updated_at,omitempty"`
	Idempotent    bool    `json:"idempotent,omitempty"`
	Message       string  `json:"message,omitempty"`
	Sandbox       bool    `json:"sandbox,omitempty"`
	Meta          *ResponseMeta
}

// CryptoWithdrawalsList is returned by [CryptoResource.Withdrawals].
type CryptoWithdrawalsList struct {
	Data       []CryptoWithdrawal `json:"data"`
	Pagination Pagination         `json:"pagination"`
	Meta       *ResponseMeta
}

// WithdrawalAddress represents a whitelisted withdrawal address.
type WithdrawalAddress struct {
	AddressUUID          string  `json:"address_uuid"`
	Currency             string  `json:"currency"`
	Network              string  `json:"network"`
	Address              string  `json:"address"`
	AddressTag           *string `json:"address_tag,omitempty"`
	Label                string  `json:"label"`
	Status               string  `json:"status"`
	CoolingUntil         *string `json:"cooling_until"`
	IsUsable             bool    `json:"is_usable"`
	WithdrawalCount      int     `json:"withdrawal_count"`
	TotalWithdrawnAmount string  `json:"total_withdrawn_amount"`
	LastWithdrawalAt     *string `json:"last_withdrawal_at,omitempty"`
	CreatedAt            string  `json:"created_at"`
}

// WithdrawalAddressesList is returned by [CryptoAddressesResource.List].
type WithdrawalAddressesList struct {
	Data       []WithdrawalAddress `json:"data"`
	Pagination Pagination          `json:"pagination"`
	Meta       *ResponseMeta
}

// AddressAddResponse is returned by [CryptoAddressesResource.Add].
type AddressAddResponse struct {
	Success     bool   `json:"success"`
	AddressUUID string `json:"address_uuid,omitempty"`
	Status      string `json:"status"` // "pending" | "active"
	Message     string `json:"message,omitempty"`
	Meta        *ResponseMeta
}

// AddressRemoveResponse is returned by [CryptoAddressesResource.Remove].
type AddressRemoveResponse struct {
	Success        bool   `json:"success"`
	AddressUUID    string `json:"address_uuid"`
	Status         string `json:"status"` // "revoked" | "pending_confirmation"
	ConfirmationID string `json:"confirmation_id,omitempty"`
	Meta           *ResponseMeta
}

// Webhook represents one registered webhook endpoint.
type Webhook struct {
	EndpointUUID   string   `json:"endpoint_uuid"`
	URL            string   `json:"url"`
	Label          string   `json:"label"`
	Events         []string `json:"events"`
	Status         string   `json:"status"`
	DisabledReason *string  `json:"disabled_reason"`
	CreatedAt      string   `json:"created_at"`
}

// WebhookCreateResponse is returned by [WebhooksResource.Create].
type WebhookCreateResponse struct {
	EndpointUUID   string `json:"endpoint_uuid,omitempty"`
	Status         string `json:"status"` // "active" | "pending_confirmation"
	SigningSecret  string `json:"signing_secret,omitempty"`
	ConfirmationID string `json:"confirmation_id,omitempty"`
	Message        string `json:"message,omitempty"`
	Meta           *ResponseMeta
}

// WebhooksListResponse is returned by [WebhooksResource.List].
type WebhooksListResponse struct {
	Endpoints []Webhook `json:"endpoints"`
	Meta      *ResponseMeta
}

// WebhookRemoveResponse is returned by [WebhooksResource.Remove].
type WebhookRemoveResponse struct {
	Success        bool   `json:"success"`
	EndpointUUID   string `json:"endpoint_uuid"`
	Status         string `json:"status"` // "deleted" | "pending_confirmation"
	ConfirmationID string `json:"confirmation_id,omitempty"`
	Meta           *ResponseMeta
}

// Instrument is one tradable instrument.
type Instrument struct {
	Instrument string `json:"instrument"`
	Base       string `json:"base"`
	Quote      string `json:"quote"`
	BaseName   string `json:"base_name"`
	Type       string `json:"type"` // "crypto_fiat" | "crypto_crypto"
}

// InstrumentsResponse is returned by [PublicResource.Instruments].
type InstrumentsResponse struct {
	Instruments []Instrument `json:"instruments"`
	Total       int          `json:"total"`
	Timestamp   string       `json:"timestamp"`
	Meta        *ResponseMeta
}

// Network is one supported network.
type Network struct {
	Coin                  string  `json:"coin"`
	Network               string  `json:"network"`
	Name                  string  `json:"name"`
	ContractAddress       *string `json:"contract_address"`
	Decimals              int     `json:"decimals"`
	MinDeposit            string  `json:"min_deposit"`
	MinWithdrawal         string  `json:"min_withdrawal"`
	WithdrawalFee         string  `json:"withdrawal_fee"`
	RequiredConfirmations int     `json:"required_confirmations"`
	DepositEnabled        bool    `json:"deposit_enabled"`
	WithdrawalEnabled     bool    `json:"withdrawal_enabled"`
}

// NetworksResponse is returned by [PublicResource.Networks].
type NetworksResponse struct {
	Networks  []Network `json:"networks"`
	Total     int       `json:"total"`
	Timestamp string    `json:"timestamp"`
	Meta      *ResponseMeta
}

// PublicFeeTier is one row in PublicFees.
type PublicFeeTier struct {
	Individual string `json:"individual,omitempty"`
	Corporate  string `json:"corporate,omitempty"`
	Note       string `json:"note,omitempty"`
}

// PublicFees is returned by [PublicResource.Fees].
type PublicFees struct {
	Trading          PublicFeeTier     `json:"trading"`
	FiatWithdrawal   PublicFeeTier     `json:"fiat_withdrawal"`
	CryptoWithdrawal map[string]string `json:"crypto_withdrawal"`
	Timestamp        string            `json:"timestamp"`
	Meta             *ResponseMeta
}

// StreamToken is the short-lived token returned by POST /stream/token.
type StreamToken struct {
	Token     string `json:"token"`
	ExpiresIn int    `json:"expires_in"`
}

// ResponseMeta holds per-response metadata attached to every successful call.
type ResponseMeta struct {
	RequestID string
	RateLimit RateLimitInfo
	Status    int
}

// WebhookEvent is the structured event returned by [VerifyWebhook].
//
// Delivery metadata from the X-Mintarex-* headers is merged with the body
// payload into one object:
//
//   - EventType, EventID, DeliveryUUID come from headers
//   - Timestamp is the ISO timestamp from the body (the Unix-seconds timestamp
//     in X-Mintarex-Timestamp is used only for signing)
//   - Data is the event-specific payload (body minus timestamp + sandbox)
//   - Sandbox is true if the event was emitted in sandbox mode
type WebhookEvent struct {
	EventType    string         `json:"event_type"`
	EventID      string         `json:"event_id"`
	DeliveryUUID string         `json:"delivery_uuid"`
	Timestamp    string         `json:"timestamp"`
	Sandbox      bool           `json:"sandbox"`
	Data         map[string]any `json:"data"`
}
