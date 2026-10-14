package models

import "fmt"

// TransactionCategoryBudgetPaceType represents how a monthly budget is expected to be used during the month
type TransactionCategoryBudgetPaceType byte

// Transaction category budget pace types
const (
	TRANSACTION_CATEGORY_BUDGET_PACE_TYPE_EVEN  TransactionCategoryBudgetPaceType = 0
	TRANSACTION_CATEGORY_BUDGET_PACE_TYPE_FIXED TransactionCategoryBudgetPaceType = 1
)

// TransactionCategoryBudgetMaxAmount is the largest allowed monthly budget amount (in cents)
const TransactionCategoryBudgetMaxAmount int64 = 99999999999

// TransactionCategoryBudget represents a monthly budget of a primary category, effective from StartYearMonth until a later row replaces it
type TransactionCategoryBudget struct {
	Uid             int64                             `xorm:"PK"`
	CategoryId      int64                             `xorm:"PK"`
	StartYearMonth  int32                             `xorm:"PK"`
	Amount          int64                             `xorm:"NOT NULL"`
	PaceType        TransactionCategoryBudgetPaceType `xorm:"TINYINT NOT NULL"`
	CreatedUnixTime int64
	UpdatedUnixTime int64
}

// TransactionCategoryBudgetSaveItem represents one category budget of a save request
type TransactionCategoryBudgetSaveItem struct {
	CategoryId int64                             `json:"categoryId,string" binding:"required,min=1"`
	Amount     int64                             `json:"amount" binding:"min=0"`
	PaceType   TransactionCategoryBudgetPaceType `json:"paceType" binding:"min=0,max=1"`
}

// TransactionCategoryBudgetSaveRequest represents all parameters of the category budgets save request
type TransactionCategoryBudgetSaveRequest struct {
	StartYearMonth string                               `json:"startYearMonth" binding:"required"`
	Budgets        []*TransactionCategoryBudgetSaveItem `json:"budgets" binding:"max=1000,dive"`
}

// TransactionCategoryBudgetInfoResponse represents a view-object of a category budget
type TransactionCategoryBudgetInfoResponse struct {
	CategoryId     int64                             `json:"categoryId,string"`
	StartYearMonth string                            `json:"startYearMonth"`
	Amount         int64                             `json:"amount"`
	PaceType       TransactionCategoryBudgetPaceType `json:"paceType"`
}

// ToTransactionCategoryBudgetInfoResponse returns a view-object according to database model
func (b *TransactionCategoryBudget) ToTransactionCategoryBudgetInfoResponse() *TransactionCategoryBudgetInfoResponse {
	return &TransactionCategoryBudgetInfoResponse{
		CategoryId:     b.CategoryId,
		StartYearMonth: FormatNumericYearMonth(b.StartYearMonth),
		Amount:         b.Amount,
		PaceType:       b.PaceType,
	}
}

// FormatNumericYearMonth returns the textual year month (e.g. 2026-10) of a numeric year month (e.g. 202610)
func FormatNumericYearMonth(yearMonth int32) string {
	return fmt.Sprintf("%04d-%02d", yearMonth/100, yearMonth%100)
}
