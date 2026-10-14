package api

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mayswind/ezbookkeeping/pkg/errs"
	"github.com/mayswind/ezbookkeeping/pkg/models"
)

func TestBankMessageCategoryDataUsesVisibleSecondaryCategoriesWithPaths(t *testing.T) {
	categories := []*models.TransactionCategory{
		{CategoryId: 1, Name: "Expenses", Type: models.CATEGORY_TYPE_EXPENSE, ParentCategoryId: models.LevelOneTransactionCategoryParentId},
		{CategoryId: 2, Name: "Grocery -", Type: models.CATEGORY_TYPE_EXPENSE, ParentCategoryId: 1},
		{CategoryId: 3, Name: "Salary +", Type: models.CATEGORY_TYPE_INCOME, ParentCategoryId: 9},
		{CategoryId: 4, Name: "Hidden -", Type: models.CATEGORY_TYPE_EXPENSE, ParentCategoryId: 1, Hidden: true},
	}

	options, lookup := bankMessageCategoryData(categories, nil)

	assert.Equal(t, []bankMessageCategoryPromptItem{{Name: "Expenses > Grocery -"}}, options["expense"])
	assert.Equal(t, []bankMessageCategoryPromptItem{{Name: "Salary +"}}, options["income"])
	assert.NotContains(t, options, "transfer")

	category, source := lookup.resolve("expense", "Expenses > Grocery -", "")
	require.NotNil(t, category)
	assert.Equal(t, int64(2), category.CategoryId)
	assert.Equal(t, "ai", source)

	category, _ = lookup.resolve("expense", "Grocery -", "")
	require.NotNil(t, category)
	assert.Equal(t, int64(2), category.CategoryId)

	category, _ = lookup.resolve("expense", "Expenses > Hidden -", "")
	assert.Nil(t, category)
}

func TestBankMessageCategoryDataDistinguishesSameNamedSecondaryCategories(t *testing.T) {
	categories := []*models.TransactionCategory{
		{CategoryId: 1, Name: "Food", Type: models.CATEGORY_TYPE_EXPENSE, ParentCategoryId: models.LevelOneTransactionCategoryParentId},
		{CategoryId: 2, Name: "Car", Type: models.CATEGORY_TYPE_EXPENSE, ParentCategoryId: models.LevelOneTransactionCategoryParentId},
		{CategoryId: 3, Name: "Other", Type: models.CATEGORY_TYPE_EXPENSE, ParentCategoryId: 1},
		{CategoryId: 4, Name: "Other", Type: models.CATEGORY_TYPE_EXPENSE, ParentCategoryId: 2},
	}

	_, lookup := bankMessageCategoryData(categories, nil)

	food, _ := lookup.resolve("expense", "Food > Other", "")
	car, _ := lookup.resolve("expense", "Car > Other", "")
	ambiguous, _ := lookup.resolve("expense", "Other", "")

	require.NotNil(t, food)
	require.NotNil(t, car)
	assert.Equal(t, int64(3), food.CategoryId)
	assert.Equal(t, int64(4), car.CategoryId)
	assert.Nil(t, ambiguous)
}

func TestBankMessageCategoryDataExcludesTransferCategories(t *testing.T) {
	categories := []*models.TransactionCategory{
		{CategoryId: 1, Name: "Transfers", Type: models.CATEGORY_TYPE_TRANSFER, ParentCategoryId: models.LevelOneTransactionCategoryParentId},
		{CategoryId: 2, Name: "Transfer Out --", Type: models.CATEGORY_TYPE_TRANSFER, ParentCategoryId: 1},
	}

	options, lookup := bankMessageCategoryData(categories, nil)
	category, _ := lookup.resolve("transfer", "Transfers > Transfer Out --", "")

	assert.NotContains(t, options, "transfer")
	assert.Nil(t, category)
}

func TestBankMessageCategoryDataIncludesTrimmedAIGuidanceAndMerchants(t *testing.T) {
	categories := []*models.TransactionCategory{
		{CategoryId: 1, Name: "Food", Type: models.CATEGORY_TYPE_EXPENSE, ParentCategoryId: 9, AiGuidance: "  Talabat food orders; consider merchant, location, and amount.  "},
		{CategoryId: 2, Name: "Transport", Type: models.CATEGORY_TYPE_EXPENSE, ParentCategoryId: 9, AiGuidance: "Talabat only when the message indicates transport."},
	}
	merchants := []*models.TransactionCategoryMerchant{
		{MerchantKey: "salik", Merchant: "Salik", CategoryId: 2},
		{MerchantKey: "careem", Merchant: "Careem", CategoryId: 2},
		{MerchantKey: "deleted", Merchant: "Deleted", CategoryId: 99},
	}

	options, _ := bankMessageCategoryData(categories, merchants)
	promptData, err := json.Marshal(options["expense"])

	require.NoError(t, err)
	assert.JSONEq(t, `[
		{"name":"Food","guidance":"Talabat food orders; consider merchant, location, and amount."},
		{"name":"Transport","guidance":"Talabat only when the message indicates transport.","merchants":["Careem","Salik"]}
	]`, string(promptData))
}

func TestBankMessageCategoryLookupPrefersMerchantRules(t *testing.T) {
	categories := []*models.TransactionCategory{
		{CategoryId: 1, Name: "Food", Type: models.CATEGORY_TYPE_EXPENSE, ParentCategoryId: models.LevelOneTransactionCategoryParentId},
		{CategoryId: 2, Name: "Restaurants", Type: models.CATEGORY_TYPE_EXPENSE, ParentCategoryId: 1},
		{CategoryId: 3, Name: "Groceries", Type: models.CATEGORY_TYPE_EXPENSE, ParentCategoryId: 1},
		{CategoryId: 5, Name: "Income", Type: models.CATEGORY_TYPE_INCOME, ParentCategoryId: models.LevelOneTransactionCategoryParentId},
		{CategoryId: 4, Name: "Refunds", Type: models.CATEGORY_TYPE_INCOME, ParentCategoryId: 5},
	}
	merchants := []*models.TransactionCategoryMerchant{
		{MerchantKey: "carrefour", Merchant: "Carrefour", CategoryId: 3},
	}

	_, lookup := bankMessageCategoryData(categories, merchants)

	category, source := lookup.resolve("expense", "Food > Restaurants", "  CARREFOUR ")
	require.NotNil(t, category)
	assert.Equal(t, int64(3), category.CategoryId)
	assert.Equal(t, "merchant", source)
	assert.Equal(t, "Food > Groceries", lookup.paths[category.CategoryId])

	// a merchant rule only applies to its own transaction type
	category, source = lookup.resolve("income", "Income > Refunds", "Carrefour")
	require.NotNil(t, category)
	assert.Equal(t, int64(4), category.CategoryId)
	assert.Equal(t, "ai", source)

	category, source = lookup.resolve("expense", "Food > Restaurants", "Unknown Cafe")
	require.NotNil(t, category)
	assert.Equal(t, int64(2), category.CategoryId)
	assert.Equal(t, "ai", source)
}

func TestBuildBankMessageRecognitionRequestRendersPreviewPrompts(t *testing.T) {
	workingDirectory, workingDirectoryErr := os.Getwd()
	require.NoError(t, workingDirectoryErr)
	require.NoError(t, os.Chdir("../.."))
	defer func() {
		require.NoError(t, os.Chdir(workingDirectory))
	}()

	setting := &models.BankMessageAutomationSetting{
		Prompt: "Prefer Food for restaurant orders.",
	}
	categoryOptions := map[string][]bankMessageCategoryPromptItem{
		"expense": {
			{Name: "Food", Guidance: "Talabat restaurant orders in Dubai."},
		},
		"income": {
			{Name: "Salary", Guidance: "Monthly employer payment."},
		},
	}

	request, err := buildBankMessageRecognitionRequest(
		setting,
		"  Purchase at Talabat  ",
		time.FixedZone("Asia/Dubai", 4*60*60),
		categoryOptions,
	)

	require.Nil(t, err)
	require.NotNil(t, request)
	assert.Equal(t, "Purchase at Talabat", string(request.UserPrompt))
	assert.Contains(t, request.SystemPrompt, "Prefer Food for restaurant orders.")
	assert.Contains(t, request.SystemPrompt, `[{"name":"Food","guidance":"Talabat restaurant orders in Dubai."}]`)
	assert.Contains(t, request.SystemPrompt, `[{"name":"Salary","guidance":"Monthly employer payment."}]`)
	assert.NotContains(t, request.SystemPrompt, "{{.")
	assert.False(t, strings.Contains(request.SystemPrompt, "\r\n"))
}

func TestBankMessageTransactionTypesRejectsTransfer(t *testing.T) {
	_, _, err := bankMessageTransactionTypes("transfer")

	assert.Equal(t, errs.ErrTransactionTypeInvalid, err)
}

func TestBankMessageProcessErrorReturnsDiagnosticsForPreview(t *testing.T) {
	response := &models.BankMessageProcessResponse{
		AIPreview: &models.BankMessageAIPreview{SystemPrompt: "rendered prompt", RawResponse: "{}"},
	}

	actualResponse, err := bankMessageProcessError(response, false, errs.ErrBankMessageAccountNotIdentified)

	require.Nil(t, err)
	assert.Same(t, response, actualResponse)
	assert.Equal(t, "preview", actualResponse.Reason)
	assert.Equal(t, "account could not be identified from bank message", actualResponse.PreviewError)
	assert.NotNil(t, actualResponse.AIPreview)
}

func TestBankMessageProcessErrorStillRejectsIngestion(t *testing.T) {
	response, err := bankMessageProcessError(&models.BankMessageProcessResponse{}, true, errs.ErrBankMessageAccountNotIdentified)

	assert.Nil(t, response)
	assert.Equal(t, errs.ErrBankMessageAccountNotIdentified, err)
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
	expectedHash := sha256.Sum256([]byte("purchase at adnoc"))

	assert.Equal(t, hex.EncodeToString(expectedHash[:]), bankMessageDuplicateKey("  Purchase   AT ADNOC "))
	assert.Equal(t, bankMessageDuplicateKey("  Purchase   AT ADNOC "), bankMessageDuplicateKey("purchase at adnoc"))
	assert.NotEqual(t, bankMessageDuplicateKey("purchase at adnoc"), bankMessageDuplicateKey("purchase at salik"))
}
