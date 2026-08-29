package models

import "encoding/json"

type BankMessageOutboxStatus string

const (
	BANK_MESSAGE_OUTBOX_STATUS_QUEUED     BankMessageOutboxStatus = "queued"
	BANK_MESSAGE_OUTBOX_STATUS_PROCESSING BankMessageOutboxStatus = "processing"
	BANK_MESSAGE_OUTBOX_STATUS_RETRYING   BankMessageOutboxStatus = "retrying"
	BANK_MESSAGE_OUTBOX_STATUS_PAUSED     BankMessageOutboxStatus = "paused"
	BANK_MESSAGE_OUTBOX_STATUS_SUCCEEDED  BankMessageOutboxStatus = "succeeded"
	BANK_MESSAGE_OUTBOX_STATUS_IGNORED    BankMessageOutboxStatus = "ignored"
	BANK_MESSAGE_OUTBOX_STATUS_FAILED     BankMessageOutboxStatus = "failed"
)

const DefaultBankMessagePrompt = `The message is from a bank about a transaction.
Categorize it for personal finance tracking.
Remarks must be a few words describing what the payment was for; use "Other" when uncertain.
Use only facts in the message. Do not invent a merchant, account, amount, currency, or time.
Normalize the merchant name by removing locations and legal suffixes such as LLC.
Etisalat is an internet provider, Salik is a road toll, and ADNOC is normally fuel.
A declined, reversed, cancelled, pending, or failed transaction is not completed.`

// BankMessageAccountMapping maps text present in a bank message to an account.
type BankMessageAccountMapping struct {
	Identifier string `json:"identifier"`
	AccountId  int64  `json:"accountId,string"`
}

// BankMessageAccountMappingSlice stores bank message account mappings as JSON.
type BankMessageAccountMappingSlice []*BankMessageAccountMapping

func (m *BankMessageAccountMappingSlice) FromDB(data []byte) error {
	return json.Unmarshal(data, m)
}

func (m BankMessageAccountMappingSlice) ToDB() ([]byte, error) {
	return json.Marshal(m)
}

// BankMessageAutomationSetting stores the bank-message automation configuration for one user.
type BankMessageAutomationSetting struct {
	Uid     int64  `xorm:"PK"`
	Enabled bool   `xorm:"NOT NULL"`
	Prompt  string `xorm:"TEXT NOT NULL"`
	// FallbackAccountId is retained only for compatibility with databases created before fallback routing was removed.
	FallbackAccountId int64                          `xorm:"NOT NULL" json:"-"`
	AccountMappings   BankMessageAccountMappingSlice `xorm:"BLOB"`
	UpdatedUnixTime   int64
}

// BankMessageEnabledUser is a singleton guard which atomically enforces that only one user is enabled.
type BankMessageEnabledUser struct {
	Slot int8  `xorm:"PK"`
	Uid  int64 `xorm:"UNIQUE NOT NULL"`
}

// BankMessageAutomationSettingRequest updates bank-message automation configuration.
type BankMessageAutomationSettingRequest struct {
	Enabled         bool                           `json:"enabled"`
	Prompt          string                         `json:"prompt" binding:"max=8000"`
	AccountMappings BankMessageAccountMappingSlice `json:"accountMappings"`
}

// BankMessageAutomationSettingResponse returns bank-message automation configuration.
type BankMessageAutomationSettingResponse struct {
	Enabled         bool                           `json:"enabled"`
	Prompt          string                         `json:"prompt"`
	DefaultPrompt   string                         `json:"defaultPrompt"`
	AccountMappings BankMessageAccountMappingSlice `json:"accountMappings"`
}

// BankMessageIngestRequest is the one-field request accepted from an automation client.
type BankMessageIngestRequest struct {
	Text string `json:"text" binding:"required,max=4000"`
}

// BankMessageOutbox stores a bank SMS until background processing finishes.
type BankMessageOutbox struct {
	OutboxId int64 `xorm:"PK"`
	Uid      int64 `xorm:"UNIQUE(UQE_bankmessageoutbox_uid_hash) INDEX(IDX_bankmessageoutbox_uid_created) INDEX(IDX_bankmessageoutbox_uid_content_hash) NOT NULL"`
	// MessageHash remains unique for compatibility with existing database indexes, but is scoped to this outbox item.
	MessageHash           string                  `xorm:"VARCHAR(64) UNIQUE(UQE_bankmessageoutbox_uid_hash) NOT NULL"`
	ContentHash           string                  `xorm:"VARCHAR(64) INDEX(IDX_bankmessageoutbox_uid_content_hash) NOT NULL"`
	ReservedTransactionId int64                   `xorm:"NOT NULL"`
	Message               string                  `xorm:"TEXT NOT NULL"`
	Status                BankMessageOutboxStatus `xorm:"VARCHAR(16) INDEX(IDX_bankmessageoutbox_status_retry) NOT NULL"`
	RetryCount            int16                   `xorm:"NOT NULL"`
	NextRetryUnixTime     int64                   `xorm:"INDEX(IDX_bankmessageoutbox_status_retry) NOT NULL"`
	LastError             string                  `xorm:"TEXT NOT NULL"`
	TransactionId         int64                   `xorm:"NOT NULL"`
	CreatedUnixTime       int64                   `xorm:"INDEX(IDX_bankmessageoutbox_uid_created) NOT NULL"`
	UpdatedUnixTime       int64                   `xorm:"NOT NULL"`
}

// BankMessageIdempotencyKey suppresses repeat deliveries only for the retry window.
type BankMessageIdempotencyKey struct {
	Uid             int64  `xorm:"PK"`
	MessageHash     string `xorm:"VARCHAR(64) PK"`
	OutboxId        int64  `xorm:"NOT NULL"`
	ExpiresUnixTime int64  `xorm:"INDEX NOT NULL"`
}

type BankMessageOutboxResponse struct {
	Id              int64                   `json:"id,string"`
	Status          BankMessageOutboxStatus `json:"status"`
	RetryCount      int16                   `json:"retryCount"`
	Message         string                  `json:"message"`
	LastError       string                  `json:"lastError,omitempty"`
	TransactionId   int64                   `json:"transactionId,string,omitempty"`
	CreatedUnixTime int64                   `json:"createdUnixTime"`
	UpdatedUnixTime int64                   `json:"updatedUnixTime"`
}

func (o *BankMessageOutbox) ToResponse() *BankMessageOutboxResponse {
	return &BankMessageOutboxResponse{
		Id:              o.OutboxId,
		Status:          o.Status,
		RetryCount:      o.RetryCount,
		Message:         o.Message,
		LastError:       o.LastError,
		TransactionId:   o.TransactionId,
		CreatedUnixTime: o.CreatedUnixTime,
		UpdatedUnixTime: o.UpdatedUnixTime,
	}
}

type BankMessageAcceptedResponse struct {
	Accepted  bool                       `json:"accepted"`
	Duplicate bool                       `json:"duplicate"`
	Outbox    *BankMessageOutboxResponse `json:"outbox"`
}

// RecognizedBankMessage is the strict structured result requested from the LLM.
type RecognizedBankMessage struct {
	Amount                      string `json:"amount" jsonschema_description:"Original transaction amount as a positive decimal number"`
	Currency                    string `json:"currency" jsonschema_description:"Three-letter ISO 4217 currency code"`
	IsPurchasedHasBeenCompleted bool   `json:"isPurchasedHasBeenCompleted" jsonschema_description:"True only when the bank says the transaction completed"`
	TransactionType             string `json:"transactionType" jsonschema:"enum=income,enum=expense"`
	Category                    string `json:"category" jsonschema_description:"Exact category name from the supplied categories"`
	TransactionTime             string `json:"transactionTime" jsonschema_description:"Transaction time in YYYY-MM-DD HH:mm:ss format; use current time when absent"`
	Remark                      string `json:"remark" jsonschema_description:"A few words describing what the transaction was for"`
	StoreName                   string `json:"storeName" jsonschema_description:"Normalized merchant or counterparty name without location or legal suffixes"`
}

// BankMessageProcessResponse describes whether a bank message created a transaction.
type BankMessageProcessResponse struct {
	Created          bool                     `json:"created"`
	Reason           string                   `json:"reason,omitempty"`
	MatchedAccountId int64                    `json:"matchedAccountId,string,omitempty"`
	Recognized       *RecognizedBankMessage   `json:"recognized,omitempty"`
	Transaction      *TransactionInfoResponse `json:"transaction,omitempty"`
}
