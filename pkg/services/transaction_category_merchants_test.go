package services

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mayswind/ezbookkeeping/pkg/datastore"
	"github.com/mayswind/ezbookkeeping/pkg/models"
)

func TestSetMerchantCategoryMovesMerchantCaseInsensitively(t *testing.T) {
	c := initializeCategoryBudgetServiceTest(t)
	require.NoError(t, datastore.Container.UserDataStore.SyncStructs(new(models.TransactionCategoryMerchant)))

	_, err := TransactionCategoryMerchants.SetMerchantCategory(c, 1, "  Carrefour   Market ", 10)
	require.NoError(t, err)

	rule, err := TransactionCategoryMerchants.SetMerchantCategory(c, 1, "CARREFOUR MARKET", 20)
	require.NoError(t, err)
	assert.Equal(t, int64(20), rule.CategoryId)

	_, err = TransactionCategoryMerchants.SetMerchantCategory(c, 2, "Carrefour Market", 30)
	require.NoError(t, err)

	merchants, err := TransactionCategoryMerchants.GetAllMerchantsByUid(c, 1)
	require.NoError(t, err)
	require.Len(t, merchants, 1)
	assert.Equal(t, "carrefour market", merchants[0].MerchantKey)
	assert.Equal(t, "CARREFOUR MARKET", merchants[0].Merchant)
	assert.Equal(t, int64(20), merchants[0].CategoryId)

	require.NoError(t, TransactionCategoryMerchants.DeleteMerchant(c, 1, "carrefour market"))
	merchants, err = TransactionCategoryMerchants.GetAllMerchantsByUid(c, 1)
	require.NoError(t, err)
	assert.Empty(t, merchants)

	_, err = TransactionCategoryMerchants.SetMerchantCategory(c, 1, "   ", 10)
	assert.Error(t, err)
}
