package errs

import "net/http"

// Error codes related to transaction category budgets
var (
	ErrTransactionCategoryBudgetMonthInvalid      = NewNormalError(NormalSubcategoryCategory, 12, http.StatusBadRequest, "budget start month is invalid")
	ErrTransactionCategoryBudgetCategoryInvalid   = NewNormalError(NormalSubcategoryCategory, 13, http.StatusBadRequest, "budgets can only be set for primary income or expense categories")
	ErrTransactionCategoryBudgetCategoryDuplicate = NewNormalError(NormalSubcategoryCategory, 14, http.StatusBadRequest, "a category appears more than once in the budgets")
	ErrTransactionCategoryBudgetAmountInvalid     = NewNormalError(NormalSubcategoryCategory, 15, http.StatusBadRequest, "budget amount is invalid")
)

// Error codes related to transaction category merchant rules
var (
	ErrTransactionCategoryMerchantInvalid         = NewNormalError(NormalSubcategoryCategory, 16, http.StatusBadRequest, "merchant name is invalid")
	ErrTransactionCategoryMerchantCategoryInvalid = NewNormalError(NormalSubcategoryCategory, 17, http.StatusBadRequest, "merchants can only be added to income or expense secondary categories")
)
