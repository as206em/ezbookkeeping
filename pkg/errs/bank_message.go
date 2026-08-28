package errs

import "net/http"

var (
	ErrBankMessageAutomationNotEnabled = NewNormalError(NormalSubcategoryLargeLanguageModel, 7, http.StatusNotFound, "bank message automation is not enabled")
	ErrBankMessageAutomationAmbiguous  = NewNormalError(NormalSubcategoryLargeLanguageModel, 8, http.StatusConflict, "bank message automation is enabled for more than one user")
	ErrBankMessageAccountNotIdentified = NewNormalError(NormalSubcategoryLargeLanguageModel, 9, http.StatusUnprocessableEntity, "account could not be identified from bank message")
	ErrBankMessageAccountAmbiguous     = NewNormalError(NormalSubcategoryLargeLanguageModel, 10, http.StatusUnprocessableEntity, "bank message matches more than one account")
	ErrBankMessageCategoryNotFound     = NewNormalError(NormalSubcategoryLargeLanguageModel, 11, http.StatusUnprocessableEntity, "recognized transaction category does not exist")
	ErrBankMessageCurrencyNotSupported = NewNormalError(NormalSubcategoryLargeLanguageModel, 12, http.StatusUnprocessableEntity, "recognized transaction currency cannot be converted to the account currency")
	ErrBankMessageAlreadyEnabled       = NewNormalError(NormalSubcategoryLargeLanguageModel, 13, http.StatusConflict, "bank message automation is already enabled for another user")
)
