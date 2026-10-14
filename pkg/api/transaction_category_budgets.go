package api

import (
	"github.com/mayswind/ezbookkeeping/pkg/core"
	"github.com/mayswind/ezbookkeeping/pkg/errs"
	"github.com/mayswind/ezbookkeeping/pkg/log"
	"github.com/mayswind/ezbookkeeping/pkg/models"
	"github.com/mayswind/ezbookkeeping/pkg/services"
	"github.com/mayswind/ezbookkeeping/pkg/utils"
)

// TransactionCategoryBudgetsApi represents transaction category budget api
type TransactionCategoryBudgetsApi struct {
	budgets    *services.TransactionCategoryBudgetService
	categories *services.TransactionCategoryService
}

// Initialize a transaction category budget api singleton instance
var (
	TransactionCategoryBudgets = &TransactionCategoryBudgetsApi{
		budgets:    services.TransactionCategoryBudgets,
		categories: services.TransactionCategories,
	}
)

// BudgetListHandler returns the full budget history of current user
func (a *TransactionCategoryBudgetsApi) BudgetListHandler(c *core.WebContext) (any, *errs.Error) {
	uid := c.GetCurrentUid()
	budgets, err := a.budgets.GetAllBudgetsByUid(c, uid)

	if err != nil {
		log.Errorf(c, "[transaction_category_budgets.BudgetListHandler] failed to get budgets for user \"uid:%d\", because %s", uid, err.Error())
		return nil, errs.Or(err, errs.ErrOperationFailed)
	}

	return a.getBudgetListResponse(budgets), nil
}

// BudgetSaveHandler replaces the budgets of current user from the requested month onward
func (a *TransactionCategoryBudgetsApi) BudgetSaveHandler(c *core.WebContext) (any, *errs.Error) {
	var saveReq models.TransactionCategoryBudgetSaveRequest
	err := c.ShouldBindJSON(&saveReq)

	if err != nil {
		log.Warnf(c, "[transaction_category_budgets.BudgetSaveHandler] parse request failed, because %s", err.Error())
		return nil, errs.NewIncompleteOrIncorrectSubmissionError(err)
	}

	startYearMonth, err := parseBudgetStartYearMonth(saveReq.StartYearMonth)

	if err != nil {
		return nil, errs.ErrTransactionCategoryBudgetMonthInvalid
	}

	uid := c.GetCurrentUid()
	categoryIds := make([]int64, 0, len(saveReq.Budgets))
	existedCategoryIds := make(map[int64]bool, len(saveReq.Budgets))

	for _, item := range saveReq.Budgets {
		if item == nil {
			return nil, errs.ErrIncompleteOrIncorrectSubmission
		}

		if existedCategoryIds[item.CategoryId] {
			return nil, errs.ErrTransactionCategoryBudgetCategoryDuplicate
		}

		if item.Amount < 0 || item.Amount > models.TransactionCategoryBudgetMaxAmount {
			return nil, errs.ErrTransactionCategoryBudgetAmountInvalid
		}

		existedCategoryIds[item.CategoryId] = true
		categoryIds = append(categoryIds, item.CategoryId)
	}

	if len(categoryIds) > 0 {
		categoryMap, err := a.categories.GetCategoriesByCategoryIds(c, uid, categoryIds)

		if err != nil {
			log.Errorf(c, "[transaction_category_budgets.BudgetSaveHandler] failed to get categories for user \"uid:%d\", because %s", uid, err.Error())
			return nil, errs.Or(err, errs.ErrOperationFailed)
		}

		for _, categoryId := range categoryIds {
			if !isBudgetableCategory(categoryMap[categoryId]) {
				return nil, errs.ErrTransactionCategoryBudgetCategoryInvalid
			}
		}
	}

	budgets := make([]*models.TransactionCategoryBudget, len(saveReq.Budgets))

	for i, item := range saveReq.Budgets {
		budgets[i] = &models.TransactionCategoryBudget{
			Uid:        uid,
			CategoryId: item.CategoryId,
			Amount:     item.Amount,
			PaceType:   item.PaceType,
		}
	}

	err = a.budgets.SaveBudgets(c, uid, startYearMonth, budgets)

	if err != nil {
		log.Errorf(c, "[transaction_category_budgets.BudgetSaveHandler] failed to save budgets for user \"uid:%d\", because %s", uid, err.Error())
		return nil, errs.Or(err, errs.ErrOperationFailed)
	}

	log.Infof(c, "[transaction_category_budgets.BudgetSaveHandler] user \"uid:%d\" has saved %d budgets from %s", uid, len(budgets), saveReq.StartYearMonth)

	allBudgets, err := a.budgets.GetAllBudgetsByUid(c, uid)

	if err != nil {
		log.Errorf(c, "[transaction_category_budgets.BudgetSaveHandler] failed to get budgets for user \"uid:%d\", because %s", uid, err.Error())
		return nil, errs.Or(err, errs.ErrOperationFailed)
	}

	return a.getBudgetListResponse(allBudgets), nil
}

func (a *TransactionCategoryBudgetsApi) getBudgetListResponse(budgets []*models.TransactionCategoryBudget) []*models.TransactionCategoryBudgetInfoResponse {
	responses := make([]*models.TransactionCategoryBudgetInfoResponse, len(budgets))

	for i, budget := range budgets {
		responses[i] = budget.ToTransactionCategoryBudgetInfoResponse()
	}

	return responses
}

func parseBudgetStartYearMonth(textualYearMonth string) (int32, error) {
	year, month, err := utils.ParseNumericYearMonth(textualYearMonth)

	if err != nil {
		return 0, err
	}

	if year < 1900 || year > 9999 || month < 1 || month > 12 {
		return 0, errs.ErrTransactionCategoryBudgetMonthInvalid
	}

	return year*100 + month, nil
}

func isBudgetableCategory(category *models.TransactionCategory) bool {
	if category == nil || category.Deleted {
		return false
	}

	if category.ParentCategoryId != models.LevelOneTransactionCategoryParentId {
		return false
	}

	return category.Type == models.CATEGORY_TYPE_EXPENSE || category.Type == models.CATEGORY_TYPE_INCOME
}
