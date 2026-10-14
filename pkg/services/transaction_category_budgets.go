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

// SaveBudgets replaces the budgets of a user from the given month onward.
// Budgets of earlier months are kept, so reports of past months still use the budgets that applied then.
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

		_, err = sess.Where("uid=? AND start_year_month>=?", uid, startYearMonth).Delete(&models.TransactionCategoryBudget{})

		if err != nil {
			return err
		}

		previous := getEffectiveBudgets(earlierBudgets)
		desired := make(map[int64]*models.TransactionCategoryBudget, len(budgets))

		for _, budget := range budgets {
			if budget.Amount > 0 {
				desired[budget.CategoryId] = budget
			}
		}

		changed := make([]*models.TransactionCategoryBudget, 0, len(desired))

		for categoryId, budget := range desired {
			if old, exists := previous[categoryId]; exists && old.Amount == budget.Amount && old.PaceType == budget.PaceType {
				continue
			}

			changed = append(changed, &models.TransactionCategoryBudget{
				Uid:             uid,
				CategoryId:      categoryId,
				StartYearMonth:  startYearMonth,
				Amount:          budget.Amount,
				PaceType:        budget.PaceType,
				CreatedUnixTime: now,
				UpdatedUnixTime: now,
			})
		}

		for categoryId, old := range previous {
			if _, exists := desired[categoryId]; exists || old.Amount <= 0 {
				continue
			}

			changed = append(changed, &models.TransactionCategoryBudget{
				Uid:             uid,
				CategoryId:      categoryId,
				StartYearMonth:  startYearMonth,
				Amount:          0,
				PaceType:        old.PaceType,
				CreatedUnixTime: now,
				UpdatedUnixTime: now,
			})
		}

		for _, budget := range changed {
			if _, err := sess.Insert(budget); err != nil {
				return err
			}
		}

		return nil
	})
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
