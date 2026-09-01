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

func TestBankMessageCategoryDataUsesVisibleSecondaryCategories(t *testing.T) {
	categories := []*models.TransactionCategory{
		{CategoryId: 1, Name: "Expenses", Type: models.CATEGORY_TYPE_EXPENSE, ParentCategoryId: models.LevelOneTransactionCategoryParentId},
		{CategoryId: 2, Name: "Grocery -", Type: models.CATEGORY_TYPE_EXPENSE, ParentCategoryId: 1},
		{CategoryId: 3, Name: "Salary +", Type: models.CATEGORY_TYPE_INCOME, ParentCategoryId: 9},
		{CategoryId: 4, Name: "Hidden -", Type: models.CATEGORY_TYPE_EXPENSE, ParentCategoryId: 1, Hidden: true},
	}

	options, categoryMap := bankMessageCategoryData(categories)

	assert.Equal(t, []bankMessageCategoryPromptItem{{Name: "Grocery -"}}, options["expense"])
	assert.Equal(t, []bankMessageCategoryPromptItem{{Name: "Salary +"}}, options["income"])
	assert.NotContains(t, options, "transfer")
	assert.Equal(t, int64(2), categoryMap["expense\x00Grocery -"].CategoryId)
	assert.NotContains(t, categoryMap, "expense\x00Hidden -")
}

func TestBankMessageCategoryDataExcludesTransferCategories(t *testing.T) {
	categories := []*models.TransactionCategory{
		{CategoryId: 1, Name: "Transfers", Type: models.CATEGORY_TYPE_TRANSFER, ParentCategoryId: models.LevelOneTransactionCategoryParentId},
		{CategoryId: 2, Name: "Transfer Out --", Type: models.CATEGORY_TYPE_TRANSFER, ParentCategoryId: 1},
	}

	options, categoryMap := bankMessageCategoryData(categories)

	assert.NotContains(t, options, "transfer")
	assert.NotContains(t, categoryMap, "transfer\x00Transfer Out --")
}

func TestBankMessageCategoryDataIncludesTrimmedAIGuidance(t *testing.T) {
	categories := []*models.TransactionCategory{
		{CategoryId: 1, Name: "Food", Type: models.CATEGORY_TYPE_EXPENSE, ParentCategoryId: 9, AiGuidance: "  Talabat food orders; consider merchant, location, and amount.  "},
		{CategoryId: 2, Name: "Transport", Type: models.CATEGORY_TYPE_EXPENSE, ParentCategoryId: 9, AiGuidance: "Talabat only when the message indicates transport."},
	}

	options, _ := bankMessageCategoryData(categories)
	promptData, err := json.Marshal(options["expense"])

	require.NoError(t, err)
	assert.Equal(t, []bankMessageCategoryPromptItem{
		{Name: "Food", Guidance: "Talabat food orders; consider merchant, location, and amount."},
		{Name: "Transport", Guidance: "Talabat only when the message indicates transport."},
	}, options["expense"])
	assert.JSONEq(t, `[
		{"name":"Food","guidance":"Talabat food orders; consider merchant, location, and amount."},
		{"name":"Transport","guidance":"Talabat only when the message indicates transport."}
	]`, string(promptData))
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
