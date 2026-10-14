package models

import (
	"strings"
	"unicode"
)

// TransactionCategoryMerchantMaxLength is the longest merchant name that can be mapped to a category
const TransactionCategoryMerchantMaxLength = 64

// TransactionCategoryMerchant maps a merchant name to the secondary category its transactions always belong to
type TransactionCategoryMerchant struct {
	Uid             int64  `xorm:"PK"`
	MerchantKey     string `xorm:"VARCHAR(64) PK"`
	Merchant        string `xorm:"VARCHAR(64) NOT NULL"`
	CategoryId      int64  `xorm:"INDEX NOT NULL"`
	CreatedUnixTime int64
	UpdatedUnixTime int64
}

// TransactionCategoryMerchantAddRequest represents all parameters of the merchant rule adding request
type TransactionCategoryMerchantAddRequest struct {
	CategoryId int64  `json:"categoryId,string" binding:"required,min=1"`
	Merchant   string `json:"merchant" binding:"required,notBlank,max=64"`
}

// TransactionCategoryMerchantDeleteRequest represents all parameters of the merchant rule deleting request
type TransactionCategoryMerchantDeleteRequest struct {
	Merchant string `json:"merchant" binding:"required,notBlank,max=64"`
}

// TransactionCategoryMerchantInfoResponse represents a view-object of a merchant rule
type TransactionCategoryMerchantInfoResponse struct {
	Merchant   string `json:"merchant"`
	CategoryId int64  `json:"categoryId,string"`
}

// ToTransactionCategoryMerchantInfoResponse returns a view-object according to database model
func (m *TransactionCategoryMerchant) ToTransactionCategoryMerchantInfoResponse() *TransactionCategoryMerchantInfoResponse {
	return &TransactionCategoryMerchantInfoResponse{
		Merchant:   m.Merchant,
		CategoryId: m.CategoryId,
	}
}

// NormalizeMerchantName trims a merchant name and collapses its inner whitespace
func NormalizeMerchantName(merchant string) string {
	return strings.Join(strings.FieldsFunc(merchant, unicode.IsSpace), " ")
}

// GetMerchantKey returns the case-insensitive lookup key of a merchant name
func GetMerchantKey(merchant string) string {
	return strings.ToLower(NormalizeMerchantName(merchant))
}
