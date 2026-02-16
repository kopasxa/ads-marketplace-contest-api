package dto

type AuthResponse struct {
	Token string `json:"token"`
	User  any    `json:"user"`
}

type ErrorResponse struct {
	Error     string `json:"error"`
	RequestID string `json:"request_id,omitempty"`
}

type SuccessResponse struct {
	OK   bool `json:"ok"`
	Data any  `json:"data,omitempty"`
}

type PaymentInfoResponse struct {
	DealID        string `json:"deal_id"`
	WalletAddress string `json:"wallet_address"`
	Memo          string `json:"memo"`
	AmountTON     string `json:"amount_ton"`
	Status        string `json:"status"`
}

type BotInviteResponse struct {
	Instructions string `json:"instructions"`
}
