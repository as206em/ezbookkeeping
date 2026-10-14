package services

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mayswind/ezbookkeeping/pkg/core"
	"github.com/mayswind/ezbookkeeping/pkg/datastore"
	"github.com/mayswind/ezbookkeeping/pkg/models"
	"github.com/mayswind/ezbookkeeping/pkg/settings"
)

func initializeCategoryBudgetServiceTest(t *testing.T) core.Context {
	t.Helper()

	config := &settings.Config{
		DatabaseConfig: &settings.DatabaseConfig{
			DatabaseType:          settings.Sqlite3DbType,
			DatabasePath:          filepath.Join(t.TempDir(), "category-budget-test.db"),
			MaxIdleConnection:     4,
			MaxOpenConnection:     4,
			ConnectionMaxLifeTime: 60,
		},
	}

	require.NoError(t, datastore.InitializeDataStore(config))
	t.Cleanup(func() {
		require.NoError(t, datastore.Container.UserStore.Choose(0).Close())
	})
	require.NoError(t, datastore.Container.UserDataStore.SyncStructs(new(models.TransactionCategoryBudget)))

	return core.NewNullContext()
}

func budgetMap(budgets []*models.TransactionCategoryBudget) map[int32]map[int64]int64 {
	result := make(map[int32]map[int64]int64)

	for _, budget := range budgets {
		if result[budget.StartYearMonth] == nil {
			result[budget.StartYearMonth] = make(map[int64]int64)
		}

		result[budget.StartYearMonth][budget.CategoryId] = budget.Amount
	}

	return result
}

func TestSaveBudgetsKeepsHistoryOfEarlierMonths(t *testing.T) {
	c := initializeCategoryBudgetServiceTest(t)

	require.NoError(t, TransactionCategoryBudgets.SaveBudgets(c, 1, 202608, []*models.TransactionCategoryBudget{
		{CategoryId: 10, Amount: 90000},
		{CategoryId: 20, Amount: 220000, PaceType: models.TRANSACTION_CATEGORY_BUDGET_PACE_TYPE_FIXED},
	}))

	require.NoError(t, TransactionCategoryBudgets.SaveBudgets(c, 1, 202610, []*models.TransactionCategoryBudget{
		{CategoryId: 10, Amount: 100000},
		{CategoryId: 20, Amount: 220000, PaceType: models.TRANSACTION_CATEGORY_BUDGET_PACE_TYPE_FIXED},
		{CategoryId: 30, Amount: 0},
	}))

	budgets, err := TransactionCategoryBudgets.GetAllBudgetsByUid(c, 1)
	require.NoError(t, err)

	assert.Equal(t, map[int32]map[int64]int64{
		202608: {10: 90000, 20: 220000},
		202610: {10: 100000},
	}, budgetMap(budgets))
}

func TestSaveBudgetsRecordsRemovalAndReplacesLaterMonths(t *testing.T) {
	c := initializeCategoryBudgetServiceTest(t)

	require.NoError(t, TransactionCategoryBudgets.SaveBudgets(c, 1, 202605, []*models.TransactionCategoryBudget{
		{CategoryId: 10, Amount: 50000},
		{CategoryId: 20, Amount: 30000},
	}))
	require.NoError(t, TransactionCategoryBudgets.SaveBudgets(c, 1, 202610, []*models.TransactionCategoryBudget{
		{CategoryId: 10, Amount: 70000},
		{CategoryId: 20, Amount: 30000},
	}))

	// saving from an earlier month replaces everything from that month onward
	require.NoError(t, TransactionCategoryBudgets.SaveBudgets(c, 1, 202607, []*models.TransactionCategoryBudget{
		{CategoryId: 10, Amount: 60000},
	}))

	budgets, err := TransactionCategoryBudgets.GetAllBudgetsByUid(c, 1)
	require.NoError(t, err)

	assert.Equal(t, map[int32]map[int64]int64{
		202605: {10: 50000, 20: 30000},
		202607: {10: 60000, 20: 0},
	}, budgetMap(budgets))

	otherUserBudgets, err := TransactionCategoryBudgets.GetAllBudgetsByUid(c, 2)
	require.NoError(t, err)
	assert.Empty(t, otherUserBudgets)

	require.NoError(t, TransactionCategoryBudgets.DeleteAllBudgets(c, 1))
	budgets, err = TransactionCategoryBudgets.GetAllBudgetsByUid(c, 1)
	require.NoError(t, err)
	assert.Empty(t, budgets)
}
