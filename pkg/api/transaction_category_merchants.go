package api

import (
	"github.com/mayswind/ezbookkeeping/pkg/core"
	"github.com/mayswind/ezbookkeeping/pkg/errs"
	"github.com/mayswind/ezbookkeeping/pkg/log"
	"github.com/mayswind/ezbookkeeping/pkg/models"
	"github.com/mayswind/ezbookkeeping/pkg/services"
)

// TransactionCategoryMerchantsApi represents transaction category merchant rule api
type TransactionCategoryMerchantsApi struct {
	merchants  *services.TransactionCategoryMerchantService
	categories *services.TransactionCategoryService
}

// Initialize a transaction category merchant rule api singleton instance
var (
	TransactionCategoryMerchants = &TransactionCategoryMerchantsApi{
		merchants:  services.TransactionCategoryMerchants,
		categories: services.TransactionCategories,
	}
)

// MerchantListHandler returns all merchant rules of current user
func (a *TransactionCategoryMerchantsApi) MerchantListHandler(c *core.WebContext) (any, *errs.Error) {
	uid := c.GetCurrentUid()
	merchants, err := a.merchants.GetAllMerchantsByUid(c, uid)

	if err != nil {
		log.Errorf(c, "[transaction_category_merchants.MerchantListHandler] failed to get merchant rules for user \"uid:%d\", because %s", uid, err.Error())
		return nil, errs.Or(err, errs.ErrOperationFailed)
	}

	responses := make([]*models.TransactionCategoryMerchantInfoResponse, len(merchants))

	for i, merchant := range merchants {
		responses[i] = merchant.ToTransactionCategoryMerchantInfoResponse()
	}

	return responses, nil
}

// MerchantAddHandler maps a merchant to a category for current user, moving it from its previous category
func (a *TransactionCategoryMerchantsApi) MerchantAddHandler(c *core.WebContext) (any, *errs.Error) {
	var addReq models.TransactionCategoryMerchantAddRequest
	err := c.ShouldBindJSON(&addReq)

	if err != nil {
		log.Warnf(c, "[transaction_category_merchants.MerchantAddHandler] parse request failed, because %s", err.Error())
		return nil, errs.NewIncompleteOrIncorrectSubmissionError(err)
	}

	uid := c.GetCurrentUid()
	category, err := a.categories.GetCategoryByCategoryId(c, uid, addReq.CategoryId)

	if err != nil {
		return nil, errs.Or(err, errs.ErrTransactionCategoryNotFound)
	}

	if category.ParentCategoryId == models.LevelOneTransactionCategoryParentId ||
		(category.Type != models.CATEGORY_TYPE_EXPENSE && category.Type != models.CATEGORY_TYPE_INCOME) {
		return nil, errs.ErrTransactionCategoryMerchantCategoryInvalid
	}

	merchant, err := a.merchants.SetMerchantCategory(c, uid, addReq.Merchant, category.CategoryId)

	if err != nil {
		log.Errorf(c, "[transaction_category_merchants.MerchantAddHandler] failed to add merchant rule for user \"uid:%d\", because %s", uid, err.Error())
		return nil, errs.Or(err, errs.ErrOperationFailed)
	}

	return merchant.ToTransactionCategoryMerchantInfoResponse(), nil
}

// MerchantDeleteHandler removes a merchant rule of current user
func (a *TransactionCategoryMerchantsApi) MerchantDeleteHandler(c *core.WebContext) (any, *errs.Error) {
	var deleteReq models.TransactionCategoryMerchantDeleteRequest
	err := c.ShouldBindJSON(&deleteReq)

	if err != nil {
		log.Warnf(c, "[transaction_category_merchants.MerchantDeleteHandler] parse request failed, because %s", err.Error())
		return nil, errs.NewIncompleteOrIncorrectSubmissionError(err)
	}

	uid := c.GetCurrentUid()

	if err := a.merchants.DeleteMerchant(c, uid, deleteReq.Merchant); err != nil {
		log.Errorf(c, "[transaction_category_merchants.MerchantDeleteHandler] failed to delete merchant rule for user \"uid:%d\", because %s", uid, err.Error())
		return nil, errs.Or(err, errs.ErrOperationFailed)
	}

	return true, nil
}
