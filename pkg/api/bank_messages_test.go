package api

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mayswind/ezbookkeeping/pkg/errs"
	"github.com/mayswind/ezbookkeeping/pkg/models"
)

func TestBankMessageCategoryDataUsesVisibleSecondaryCategories(t *testing.T) {
	categories := []*models.TransactionCategory{
		{CategoryId: 1, Name: "Expenses", Type: models.CATEGORY_TYPE_EXPENSE, ParentCategoryId: models.LevelOneTransactionCategoryParentId},
		{CategoryId: 2, Name: "Grocery -", Type: models.CATEGORY_TYPE_EXPENSE, ParentCategoryId: 1},
		{CategoryId: 3, Name: "Salary +", Type: models.CATEGORY_TYPE_INCOME, ParentCategoryId: 9},
		{CategoryId: 4, Name: "Hidden -", Type: models.CATEGORY_TYPE_EXPENSE, ParentCategoryId: 1, Hidden: true},
	}

	names, categoryMap := bankMessageCategoryData(categories)

	assert.Equal(t, []string{"Grocery -"}, names["expense"])
	assert.Equal(t, []string{"Salary +"}, names["income"])
	assert.NotContains(t, names, "transfer")
	assert.Equal(t, int64(2), categoryMap["expense\x00Grocery -"].CategoryId)
	assert.NotContains(t, categoryMap, "expense\x00Hidden -")
}

func TestBankMessageCategoryDataExcludesTransferCategories(t *testing.T) {
	categories := []*models.TransactionCategory{
		{CategoryId: 1, Name: "Transfers", Type: models.CATEGORY_TYPE_TRANSFER, ParentCategoryId: models.LevelOneTransactionCategoryParentId},
		{CategoryId: 2, Name: "Transfer Out --", Type: models.CATEGORY_TYPE_TRANSFER, ParentCategoryId: 1},
	}

	names, categoryMap := bankMessageCategoryData(categories)

	assert.NotContains(t, names, "transfer")
	assert.NotContains(t, categoryMap, "transfer\x00Transfer Out --")
}

func TestBankMessageTransactionTypesRejectsTransfer(t *testing.T) {
	_, _, err := bankMessageTransactionTypes("transfer")

	assert.Equal(t, errs.ErrTransactionTypeInvalid, err)
}

func TestIdentifyAccountUsesOneMatchingRule(t *testing.T) {
	setting := &models.BankMessageAutomationSetting{
		AccountMappings: models.BankMessageAccountMappingSlice{
			{Identifier: "card ending 1234", AccountId: 10},
			{Identifier: "card ending 7788", AccountId: 20},
		},
	}

	accountId, err := BankMessages.identifyAccount(nil, setting, "Purchase using CARD ENDING 1234 at ADNOC")

	require.Nil(t, err)
	assert.Equal(t, int64(10), accountId)
}

func TestIdentifyAccountRejectsWhenNoRuleMatches(t *testing.T) {
	setting := &models.BankMessageAutomationSetting{}

	accountId, err := BankMessages.identifyAccount(nil, setting, "Purchase at ADNOC")

	assert.Equal(t, int64(0), accountId)
	assert.Equal(t, errs.ErrBankMessageAccountNotIdentified, err)
}

func TestIdentifyAccountRejectsAmbiguousRules(t *testing.T) {
	setting := &models.BankMessageAutomationSetting{
		AccountMappings: models.BankMessageAccountMappingSlice{
			{Identifier: "1234", AccountId: 10},
			{Identifier: "ADNOC", AccountId: 20},
		},
	}

	_, err := BankMessages.identifyAccount(nil, setting, "Card 1234 purchase at ADNOC")

	assert.Equal(t, errs.ErrBankMessageAccountAmbiguous, err)
}

func TestConvertBankMessageAmount(t *testing.T) {
	converted, ok := convertBankMessageAmount(10000, "1", "3.6725")

	require.True(t, ok)
	assert.Equal(t, int64(36725), converted)
}

func TestBankMessageComment(t *testing.T) {
	assert.Equal(t, "ADNOC - Fuel purchase", bankMessageComment("ADNOC", "Fuel purchase"))
	assert.Equal(t, "ADNOC", bankMessageComment("ADNOC", "adnoc"))
	assert.Len(t, []rune(bankMessageComment(string(make([]rune, 300)), "")), 255)
}

func TestBankMessageDuplicateKeyNormalizesCaseAndWhitespace(t *testing.T) {
	assert.Equal(t, bankMessageDuplicateKey("  Purchase   AT ADNOC "), bankMessageDuplicateKey("purchase at adnoc"))
	assert.NotEqual(t, bankMessageDuplicateKey("purchase at adnoc"), bankMessageDuplicateKey("purchase at salik"))
}
