package services

import (
	"time"

	"xorm.io/xorm"

	"github.com/mayswind/ezbookkeeping/pkg/core"
	"github.com/mayswind/ezbookkeeping/pkg/datastore"
	"github.com/mayswind/ezbookkeeping/pkg/errs"
	"github.com/mayswind/ezbookkeeping/pkg/models"
)

// TransactionCategoryBudgetService represents transaction category budget service
type TransactionCategoryBudgetService struct {
	ServiceUsingDB
}

// Initialize a transaction category budget service singleton instance
var (
	TransactionCategoryBudgets = &TransactionCategoryBudgetService{
		ServiceUsingDB: ServiceUsingDB{
			container: datastore.Container,
		},
	}
)

// GetAllBudgetsByUid returns the full budget history of a user, ordered by start month
func (s *TransactionCategoryBudgetService) GetAllBudgetsByUid(c core.Context, uid int64) ([]*models.TransactionCategoryBudget, error) {
	if uid <= 0 {
		return nil, errs.ErrUserIdInvalid
	}

	var budgets []*models.TransactionCategoryBudget
	err := s.UserDataDB(uid).NewSession(c).Where("uid=?", uid).OrderBy("start_year_month asc, category_id asc").Find(&budgets)

	return budgets, err
}

// SaveBudgets sets the budgets of a user from the given month onward.
// Only categories whose budget differs from the one already in effect in that month are changed: their rows from that
// month onward are replaced. Other categories keep their later changes, and earlier months are never modified, so
// reports of past months still use the budgets that applied then.
// A category omitted from budgets (or with a zero amount) has no budget from that month.
func (s *TransactionCategoryBudgetService) SaveBudgets(c core.Context, uid int64, startYearMonth int32, budgets []*models.TransactionCategoryBudget) error {
	if uid <= 0 {
		return errs.ErrUserIdInvalid
	}

	now := time.Now().Unix()

	return s.UserDataDB(uid).DoTransaction(c, func(sess *xorm.Session) error {
		var earlierBudgets []*models.TransactionCategoryBudget
		err := sess.Where("uid=? AND start_year_month<?", uid, startYearMonth).OrderBy("start_year_month asc").Find(&earlierBudgets)

		if err != nil {
			return err
		}

		var startMonthBudgets []*models.TransactionCategoryBudget
		err = sess.Where("uid=? AND start_year_month=?", uid, startYearMonth).Find(&startMonthBudgets)

		if err != nil {
			return err
		}

		previous := getEffectiveBudgets(earlierBudgets)
		current := getEffectiveBudgets(append(append([]*models.TransactionCategoryBudget{}, earlierBudgets...), startMonthBudgets...))
		desired := make(map[int64]*models.TransactionCategoryBudget, len(budgets))

		for _, budget := range budgets {
			if budget.Amount > 0 {
				desired[budget.CategoryId] = budget
			}
		}

		categoryIds := make(map[int64]bool, len(desired)+len(current))

		for categoryId := range desired {
			categoryIds[categoryId] = true
		}

		for categoryId := range current {
			categoryIds[categoryId] = true
		}

		for categoryId := range categoryIds {
			newBudget := desired[categoryId]

			if isSameBudget(current[categoryId], newBudget) {
				continue
			}

			_, err = sess.Where("uid=? AND category_id=? AND start_year_month>=?", uid, categoryId, startYearMonth).Delete(&models.TransactionCategoryBudget{})

			if err != nil {
				return err
			}

			if isSameBudget(previous[categoryId], newBudget) {
				continue
			}

			row := &models.TransactionCategoryBudget{
				Uid:             uid,
				CategoryId:      categoryId,
				StartYearMonth:  startYearMonth,
				CreatedUnixTime: now,
				UpdatedUnixTime: now,
			}

			if newBudget != nil {
				row.Amount = newBudget.Amount
				row.PaceType = newBudget.PaceType
			} else if old := previous[categoryId]; old != nil {
				row.PaceType = old.PaceType
			}

			if _, err = sess.Insert(row); err != nil {
				return err
			}
		}

		return nil
	})
}

// isSameBudget returns whether an existing budget row (nil or zero amount means no budget) matches a new budget (nil means no budget)
func isSameBudget(existing *models.TransactionCategoryBudget, newBudget *models.TransactionCategoryBudget) bool {
	existingHasBudget := existing != nil && existing.Amount > 0

	if !existingHasBudget || newBudget == nil {
		return !existingHasBudget && newBudget == nil
	}

	return existing.Amount == newBudget.Amount && existing.PaceType == newBudget.PaceType
}

// DeleteAllBudgets deletes all budgets of a user
func (s *TransactionCategoryBudgetService) DeleteAllBudgets(c core.Context, uid int64) error {
	if uid <= 0 {
		return errs.ErrUserIdInvalid
	}

	return s.UserDataDB(uid).DoTransaction(c, func(sess *xorm.Session) error {
		_, err := sess.Where("uid=?", uid).Delete(&models.TransactionCategoryBudget{})
		return err
	})
}

// getEffectiveBudgets returns the latest budget of each category from budgets ordered by start month ascending
func getEffectiveBudgets(budgets []*models.TransactionCategoryBudget) map[int64]*models.TransactionCategoryBudget {
	effective := make(map[int64]*models.TransactionCategoryBudget, len(budgets))

	for _, budget := range budgets {
		effective[budget.CategoryId] = budget
	}

	return effective
}
