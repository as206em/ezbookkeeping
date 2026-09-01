package api

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"html/template"
	"math/big"
	"reflect"
	"sort"
	"strings"
	"time"

	"github.com/mayswind/ezbookkeeping/pkg/core"
	"github.com/mayswind/ezbookkeeping/pkg/errs"
	"github.com/mayswind/ezbookkeeping/pkg/exchangerates"
	"github.com/mayswind/ezbookkeeping/pkg/llm"
	"github.com/mayswind/ezbookkeeping/pkg/llm/data"
	"github.com/mayswind/ezbookkeeping/pkg/log"
	"github.com/mayswind/ezbookkeeping/pkg/models"
	"github.com/mayswind/ezbookkeeping/pkg/services"
	"github.com/mayswind/ezbookkeeping/pkg/settings"
	"github.com/mayswind/ezbookkeeping/pkg/templates"
	"github.com/mayswind/ezbookkeeping/pkg/utils"
)

var bankMessageDefaultTimezone = time.FixedZone("Asia/Dubai", 4*60*60)

// BankMessagesApi manages bank-message automation settings and ingestion.
type BankMessagesApi struct {
	ApiUsingConfig
	settings              *services.BankMessageSettingsService
	outbox                *services.BankMessageOutboxService
	transactionCategories *services.TransactionCategoryService
	accounts              *services.AccountService
	users                 *services.UserService
	transactions          *services.TransactionService
}

var BankMessages = &BankMessagesApi{
	ApiUsingConfig: ApiUsingConfig{
		container: settings.Container,
	},
	settings:              services.BankMessageSettings,
	outbox:                services.BankMessageOutbox,
	transactionCategories: services.TransactionCategories,
	accounts:              services.Accounts,
	users:                 services.Users,
	transactions:          services.Transactions,
}

func (a *BankMessagesApi) SettingsGetHandler(c *core.WebContext) (any, *errs.Error) {
	setting, err := a.settings.GetByUid(c, c.GetCurrentUid())

	if err != nil {
		return nil, errs.Or(err, errs.ErrOperationFailed)
	}

	if setting == nil {
		return &models.BankMessageAutomationSettingResponse{
			Prompt:          models.DefaultBankMessagePrompt,
			DefaultPrompt:   models.DefaultBankMessagePrompt,
			AccountMappings: models.BankMessageAccountMappingSlice{},
		}, nil
	}

	return a.settingResponse(setting), nil
}

func (a *BankMessagesApi) SettingsUpdateHandler(c *core.WebContext) (any, *errs.Error) {
	var request models.BankMessageAutomationSettingRequest

	if err := c.ShouldBindJSON(&request); err != nil {
		return nil, errs.NewIncompleteOrIncorrectSubmissionError(err)
	}

	uid := c.GetCurrentUid()
	prompt := strings.TrimSpace(request.Prompt)

	if prompt == "" {
		prompt = models.DefaultBankMessagePrompt
	}

	mappings, err := a.validateAndNormalizeMappings(c, uid, request.AccountMappings)

	if err != nil {
		return nil, errs.Or(err, errs.ErrIncompleteOrIncorrectSubmission)
	}

	setting := &models.BankMessageAutomationSetting{
		Uid:             uid,
		Enabled:         request.Enabled,
		Prompt:          prompt,
		AccountMappings: mappings,
	}

	if err := a.settings.Update(c, setting); err != nil {
		return nil, errs.Or(err, errs.ErrOperationFailed)
	}

	return a.settingResponse(setting), nil
}

func (a *BankMessagesApi) PreviewHandler(c *core.WebContext) (any, *errs.Error) {
	request, err := bindBankMessageRequest(c)

	if err != nil {
		return nil, err
	}

	setting, serviceErr := a.settings.GetByUid(c, c.GetCurrentUid())

	if serviceErr != nil {
		return nil, errs.Or(serviceErr, errs.ErrOperationFailed)
	} else if setting == nil {
		setting = &models.BankMessageAutomationSetting{
			Uid:     c.GetCurrentUid(),
			Prompt:  models.DefaultBankMessagePrompt,
			Enabled: false,
		}
	}

	clientTimezone, timezoneErr := c.GetClientTimezone()

	if timezoneErr != nil {
		return nil, errs.ErrClientTimezoneOffsetInvalid
	}

	return a.process(c, setting, request.Text, clientTimezone, false, 0)
}

func (a *BankMessagesApi) IngestHandler(c *core.WebContext) (any, *errs.Error) {
	request, err := bindBankMessageRequest(c)

	if err != nil {
		return nil, err
	}

	setting, serviceErr := a.settings.GetEnabled(c)

	if serviceErr != nil {
		return nil, errs.Or(serviceErr, errs.ErrOperationFailed)
	}

	item, duplicate, enqueueErr := a.outbox.Enqueue(c, setting.Uid, request.Text, bankMessageDuplicateKey(request.Text))
	if enqueueErr != nil {
		return nil, errs.Or(enqueueErr, errs.ErrOperationFailed)
	}

	return &models.BankMessageAcceptedResponse{
		Accepted:  true,
		Duplicate: duplicate,
		Outbox:    item.ToResponse(),
	}, nil
}

func (a *BankMessagesApi) OutboxListHandler(c *core.WebContext) (any, *errs.Error) {
	items, err := a.outbox.ListByUid(c, c.GetCurrentUid(), 100)
	if err != nil {
		return nil, errs.Or(err, errs.ErrOperationFailed)
	}

	responses := make([]*models.BankMessageOutboxResponse, 0, len(items))
	for _, item := range items {
		responses = append(responses, item.ToResponse())
	}

	return responses, nil
}

// ProcessOutbox processes durable bank-message work outside the ingest request.
func (a *BankMessagesApi) ProcessOutbox(c core.Context) error {
	for i := 0; i < 10; i++ {
		item, err := a.outbox.ClaimNext(c)
		if err != nil {
			return err
		} else if item == nil {
			return nil
		}

		setting, err := a.settings.GetByUid(c, item.Uid)
		if err != nil {
			if markErr := a.outbox.MarkFailedAttempt(c, item, err); markErr != nil {
				return markErr
			}
			continue
		} else if setting == nil || !setting.Enabled {
			if markErr := a.outbox.MarkPaused(c, item); markErr != nil {
				return markErr
			}
			continue
		}

		existingTransaction, getTransactionErr := a.transactions.GetTransactionByTransactionId(c, item.Uid, item.ReservedTransactionId)
		if getTransactionErr == nil {
			if err := a.outbox.MarkSucceeded(c, item, existingTransaction.TransactionId); err != nil {
				return err
			}
			continue
		} else if getTransactionErr != errs.ErrTransactionNotFound {
			if err := a.outbox.MarkFailedAttempt(c, item, getTransactionErr); err != nil {
				return err
			}
			continue
		}

		response, processErr := a.process(c, setting, item.Message, bankMessageDefaultTimezone, true, item.ReservedTransactionId)
		if processErr != nil {
			log.Errorf(c, "[bank_messages.ProcessOutbox] failed to process outbox item \"id:%d\", because %s", item.OutboxId, processErr.Error())
			if processErr == errs.ErrBankMessageAutomationNotEnabled {
				if err := a.outbox.MarkPaused(c, item); err != nil {
					return err
				}
			} else if err := a.outbox.MarkFailedAttempt(c, item, processErr); err != nil {
				return err
			}
			continue
		}

		if response != nil && response.Created && response.Transaction != nil {
			if err := a.outbox.MarkSucceeded(c, item, response.Transaction.Id); err != nil {
				return err
			}
		} else if err := a.outbox.MarkIgnored(c, item); err != nil {
			return err
		}
	}

	return nil
}

func (a *BankMessagesApi) process(c core.Context, setting *models.BankMessageAutomationSetting, text string, clientTimezone *time.Location, create bool, reservedTransactionId int64) (*models.BankMessageProcessResponse, *errs.Error) {
	config := a.CurrentConfig()

	if config.TextRecognitionLLMConfig == nil || config.TextRecognitionLLMConfig.LLMProvider == "" || !config.TransactionFromAITextRecognition {
		return nil, errs.ErrLargeLanguageModelProviderNotEnabled
	}

	user, err := a.users.GetUserById(c, setting.Uid)

	if err != nil {
		return nil, errs.ErrUserNotFound
	}

	categories, err := a.transactionCategories.GetAllCategoriesByUid(c, setting.Uid, 0, -1)

	if err != nil {
		return nil, errs.Or(err, errs.ErrOperationFailed)
	}

	categoryOptions, categoryMap := bankMessageCategoryData(categories)
	recognized, aiPreview, recognitionErr := a.recognize(c, setting, text, clientTimezone, categoryOptions)

	if recognitionErr != nil {
		return nil, recognitionErr
	}

	response := &models.BankMessageProcessResponse{
		Created:    false,
		Recognized: recognized,
	}
	if !create {
		response.AIPreview = aiPreview
	}

	if recognized.IsDeclined {
		response.Reason = "declined"
		return response, nil
	}

	category, exists := categoryMap[recognized.TransactionType+"\x00"+recognized.Category]

	if !exists {
		return bankMessageProcessError(response, create, errs.ErrBankMessageCategoryNotFound)
	}

	accountId, accountErr := a.identifyAccount(c, setting, text)

	if accountErr != nil {
		return bankMessageProcessError(response, create, accountErr)
	}

	account, err := a.accounts.GetAccountByAccountId(c, setting.Uid, accountId)

	if err != nil {
		return bankMessageProcessError(response, create, errs.Or(err, errs.ErrBankMessageAccountNotIdentified))
	}

	amount, amountErr := a.amountInAccountCurrency(c, setting.Uid, recognized.Amount, strings.ToUpper(recognized.Currency), account.Currency)

	if amountErr != nil {
		return bankMessageProcessError(response, create, amountErr)
	}

	_, dbType, typeErr := bankMessageTransactionTypes(recognized.TransactionType)

	if typeErr != nil {
		return bankMessageProcessError(response, create, typeErr)
	}

	transactionTime := time.Now().In(clientTimezone)

	if recognized.TransactionTime != "" {
		parsedTime, parseErr := utils.ParseFromLongDateTimeInTimeZone(normalizeLongDateTime(recognized.TransactionTime), clientTimezone)

		if parseErr == nil {
			transactionTime = parsedTime
		}
	}

	_, utcOffsetSeconds := transactionTime.Zone()
	comment := bankMessageComment(recognized.StoreName, recognized.Remark)
	transaction := &models.Transaction{
		Uid:               setting.Uid,
		Type:              dbType,
		CategoryId:        category.CategoryId,
		TransactionTime:   utils.GetMinTransactionTimeFromUnixTime(transactionTime.Unix()),
		TimezoneUtcOffset: int16(utcOffsetSeconds / 60),
		AccountId:         accountId,
		Amount:            amount,
		Comment:           comment,
		CreatedIp:         c.ClientIP(),
	}

	if !user.CanEditTransactionByTransactionTime(transaction.TransactionTime, clientTimezone, account, nil) {
		return bankMessageProcessError(response, create, errs.ErrCannotCreateTransactionWithThisTransactionTime)
	}

	response.MatchedAccountId = accountId

	if !create {
		response.Reason = "preview"
		return response, nil
	}

	currentSetting, err := a.settings.GetByUid(c, setting.Uid)
	if err != nil {
		return nil, errs.Or(err, errs.ErrOperationFailed)
	} else if currentSetting == nil || !currentSetting.Enabled {
		return nil, errs.ErrBankMessageAutomationNotEnabled
	}

	if err := a.transactions.CreateTransactionWithId(c, transaction, nil, nil, reservedTransactionId); err != nil {
		log.Errorf(c, "[bank_messages.process] failed to create transaction for user \"uid:%d\", because %s", setting.Uid, err.Error())
		return nil, errs.Or(err, errs.ErrOperationFailed)
	}

	response.Created = true
	response.Transaction = transaction.ToTransactionInfoResponse(nil, true)
	return response, nil
}

func bankMessageProcessError(response *models.BankMessageProcessResponse, create bool, err *errs.Error) (*models.BankMessageProcessResponse, *errs.Error) {
	if create {
		return nil, err
	}

	response.Reason = "preview"
	response.PreviewError = err.Error()
	return response, nil
}

func (a *BankMessagesApi) recognize(c core.Context, setting *models.BankMessageAutomationSetting, text string, clientTimezone *time.Location, categoryOptions map[string][]bankMessageCategoryPromptItem) (*models.RecognizedBankMessage, *models.BankMessageAIPreview, *errs.Error) {
	request, err := buildBankMessageRecognitionRequest(setting, text, clientTimezone, categoryOptions)
	if err != nil {
		return nil, nil, err
	}

	llmResponse, responseErr := llm.Container.GetJsonResponseByTextRecognitionModel(c, setting.Uid, a.CurrentConfig(), request)

	if responseErr != nil {
		return nil, nil, errs.Or(responseErr, errs.ErrOperationFailed)
	}

	if llmResponse == nil || strings.TrimSpace(llmResponse.Content) == "" {
		return nil, nil, errs.ErrNoTransactionInformation
	}

	aiPreview := &models.BankMessageAIPreview{
		SystemPrompt: request.SystemPrompt,
		UserPrompt:   string(request.UserPrompt),
		RawResponse:  llmResponse.Content,
	}
	result := &models.RecognizedBankMessage{}

	if err := json.Unmarshal([]byte(llmResponse.Content), result); err != nil {
		return nil, nil, errs.Or(err, errs.ErrOperationFailed)
	}

	result.TransactionType = strings.ToLower(strings.TrimSpace(result.TransactionType))
	result.Category = strings.TrimSpace(result.Category)
	result.Currency = strings.ToUpper(strings.TrimSpace(result.Currency))
	result.StoreName = strings.TrimSpace(result.StoreName)
	result.Remark = strings.TrimSpace(result.Remark)

	return result, aiPreview, nil
}

func buildBankMessageRecognitionRequest(setting *models.BankMessageAutomationSetting, text string, clientTimezone *time.Location, categoryOptions map[string][]bankMessageCategoryPromptItem) (*data.LargeLanguageModelRequest, *errs.Error) {
	systemPrompt, err := templates.GetTemplate(templates.SYSTEM_PROMPT_BANK_MESSAGE_RECOGNITION)

	if err != nil {
		return nil, errs.Or(err, errs.ErrOperationFailed)
	}

	expenseCategories, err := json.Marshal(categoryOptions["expense"])
	if err != nil {
		return nil, errs.Or(err, errs.ErrOperationFailed)
	}

	incomeCategories, err := json.Marshal(categoryOptions["income"])
	if err != nil {
		return nil, errs.Or(err, errs.ErrOperationFailed)
	}

	params := map[string]any{
		// These values render into an LLM text prompt, not an HTML response. Marking them as
		// template text prevents html/template from replacing JSON quotes with HTML entities.
		"CustomPrompt":         template.HTML(setting.Prompt),
		"AllExpenseCategories": template.HTML(expenseCategories),
		"AllIncomeCategories":  template.HTML(incomeCategories),
		"CurrentDateTime":      template.HTML(utils.FormatUnixTimeToLongDateTime(time.Now().Unix(), clientTimezone)),
	}

	var prompt bytes.Buffer

	if err := systemPrompt.Execute(&prompt, params); err != nil {
		return nil, errs.Or(err, errs.ErrOperationFailed)
	}

	return &data.LargeLanguageModelRequest{
		Stream:                 false,
		SystemPrompt:           strings.ReplaceAll(prompt.String(), "\r\n", "\n"),
		UserPrompt:             []byte(strings.TrimSpace(text)),
		UserPromptType:         data.LARGE_LANGUAGE_MODEL_REQUEST_PROMPT_TYPE_TEXT,
		ResponseJsonObjectType: reflect.TypeOf(models.RecognizedBankMessage{}),
	}, nil
}

func (a *BankMessagesApi) identifyAccount(c core.Context, setting *models.BankMessageAutomationSetting, text string) (int64, *errs.Error) {
	normalizedText := strings.ToLower(text)
	matchedIds := make(map[int64]bool)

	for _, mapping := range setting.AccountMappings {
		if mapping != nil && strings.Contains(normalizedText, strings.ToLower(strings.TrimSpace(mapping.Identifier))) {
			matchedIds[mapping.AccountId] = true
		}
	}

	if len(matchedIds) > 1 {
		return 0, errs.ErrBankMessageAccountAmbiguous
	}

	for accountId := range matchedIds {
		return accountId, nil
	}

	return 0, errs.ErrBankMessageAccountNotIdentified
}

func (a *BankMessagesApi) amountInAccountCurrency(c core.Context, uid int64, amountText string, fromCurrency string, toCurrency string) (int64, *errs.Error) {
	amount, err := utils.ParseAmount(amountText)

	if err != nil || amount <= 0 {
		return 0, errs.ErrBankMessageCurrencyNotSupported
	}

	if fromCurrency == toCurrency {
		return amount, nil
	}

	rates, err := exchangerates.Container.GetLatestExchangeRates(c, uid, a.CurrentConfig())

	if err != nil || rates == nil {
		return 0, errs.ErrBankMessageCurrencyNotSupported
	}

	rateMap := make(map[string]string, len(rates.ExchangeRates)+1)
	rateMap[rates.BaseCurrency] = "1"

	for _, rate := range rates.ExchangeRates {
		if rate != nil {
			rateMap[rate.Currency] = rate.Rate
		}
	}

	converted, ok := convertBankMessageAmount(amount, rateMap[fromCurrency], rateMap[toCurrency])

	if !ok {
		return 0, errs.ErrBankMessageCurrencyNotSupported
	}

	return converted, nil
}

func (a *BankMessagesApi) validateAndNormalizeMappings(c *core.WebContext, uid int64, mappings models.BankMessageAccountMappingSlice) (models.BankMessageAccountMappingSlice, error) {
	accounts, err := a.accounts.GetAllAccountsByUid(c, uid)

	if err != nil {
		return nil, err
	}

	validAccounts := make(map[int64]bool)

	for _, account := range accounts {
		if !account.Hidden && account.Type != models.ACCOUNT_TYPE_MULTI_SUB_ACCOUNTS {
			validAccounts[account.AccountId] = true
		}
	}

	seenIdentifiers := make(map[string]bool)
	normalized := make(models.BankMessageAccountMappingSlice, 0, len(mappings))

	for _, mapping := range mappings {
		if mapping == nil {
			continue
		}

		identifier := strings.TrimSpace(mapping.Identifier)
		identifierKey := strings.ToLower(identifier)

		if len([]rune(identifier)) < 4 || len([]rune(identifier)) > 64 || seenIdentifiers[identifierKey] || !validAccounts[mapping.AccountId] {
			return nil, errs.ErrParameterInvalid
		}

		seenIdentifiers[identifierKey] = true
		normalized = append(normalized, &models.BankMessageAccountMapping{
			Identifier: identifier,
			AccountId:  mapping.AccountId,
		})
	}

	sort.Slice(normalized, func(i, j int) bool {
		return strings.ToLower(normalized[i].Identifier) < strings.ToLower(normalized[j].Identifier)
	})

	return normalized, nil
}

func (a *BankMessagesApi) settingResponse(setting *models.BankMessageAutomationSetting) *models.BankMessageAutomationSettingResponse {
	return &models.BankMessageAutomationSettingResponse{
		Enabled:         setting.Enabled,
		Prompt:          setting.Prompt,
		DefaultPrompt:   models.DefaultBankMessagePrompt,
		AccountMappings: setting.AccountMappings,
	}
}

func bindBankMessageRequest(c *core.WebContext) (*models.BankMessageIngestRequest, *errs.Error) {
	request := &models.BankMessageIngestRequest{}

	if err := c.ShouldBindJSON(request); err != nil {
		return nil, errs.NewIncompleteOrIncorrectSubmissionError(err)
	}

	request.Text = strings.TrimSpace(request.Text)

	if request.Text == "" {
		return nil, errs.ErrAIRecognitionTextIsEmpty
	}

	return request, nil
}

type bankMessageCategoryPromptItem struct {
	Name     string `json:"name"`
	Guidance string `json:"guidance,omitempty"`
}

func bankMessageCategoryData(categories []*models.TransactionCategory) (map[string][]bankMessageCategoryPromptItem, map[string]*models.TransactionCategory) {
	options := map[string][]bankMessageCategoryPromptItem{
		"income":  {},
		"expense": {},
	}
	categoryMap := make(map[string]*models.TransactionCategory)

	for _, category := range categories {
		if category.Hidden || category.ParentCategoryId == models.LevelOneTransactionCategoryParentId {
			continue
		}

		typeName := ""

		switch category.Type {
		case models.CATEGORY_TYPE_INCOME:
			typeName = "income"
		case models.CATEGORY_TYPE_EXPENSE:
			typeName = "expense"
		}

		if typeName != "" {
			options[typeName] = append(options[typeName], bankMessageCategoryPromptItem{
				Name:     category.Name,
				Guidance: strings.TrimSpace(category.AiGuidance),
			})
			categoryMap[typeName+"\x00"+category.Name] = category
		}
	}

	for typeName := range options {
		sort.Slice(options[typeName], func(i int, j int) bool {
			return options[typeName][i].Name < options[typeName][j].Name
		})
	}

	return options, categoryMap
}

func bankMessageTransactionTypes(typeName string) (models.TransactionType, models.TransactionDbType, *errs.Error) {
	switch typeName {
	case "expense":
		return models.TRANSACTION_TYPE_EXPENSE, models.TRANSACTION_DB_TYPE_EXPENSE, nil
	case "income":
		return models.TRANSACTION_TYPE_INCOME, models.TRANSACTION_DB_TYPE_INCOME, nil
	default:
		return 0, 0, errs.ErrTransactionTypeInvalid
	}
}

func normalizeLongDateTime(value string) string {
	value = strings.TrimSpace(value)

	if utils.IsValidLongDateTimeWithoutSecondFormat(value) {
		return value + ":00"
	} else if utils.IsValidLongDateFormat(value) {
		return value + " 00:00:00"
	}

	return value
}

func bankMessageComment(storeName string, remark string) string {
	storeName = strings.TrimSpace(storeName)
	remark = strings.TrimSpace(remark)
	comment := storeName

	if comment != "" && remark != "" && !strings.EqualFold(comment, remark) {
		comment += " - " + remark
	} else if comment == "" {
		comment = remark
	}

	runes := []rune(comment)

	if len(runes) > 255 {
		comment = string(runes[:255])
	}

	return comment
}

func convertBankMessageAmount(amount int64, fromRateText string, toRateText string) (int64, bool) {
	fromRate, ok := new(big.Rat).SetString(fromRateText)

	if !ok || fromRate.Sign() <= 0 {
		return 0, false
	}

	toRate, ok := new(big.Rat).SetString(toRateText)

	if !ok || toRate.Sign() <= 0 {
		return 0, false
	}

	converted := new(big.Rat).Mul(new(big.Rat).SetInt64(amount), toRate)
	converted.Quo(converted, fromRate)
	numerator := new(big.Int).Set(converted.Num())
	denominator := converted.Denom()
	numerator.Add(numerator, new(big.Int).Quo(new(big.Int).Set(denominator), big.NewInt(2)))
	numerator.Quo(numerator, denominator)

	if !numerator.IsInt64() || numerator.Sign() <= 0 {
		return 0, false
	}

	return numerator.Int64(), true
}

func bankMessageDuplicateKey(text string) string {
	normalized := strings.Join(strings.Fields(strings.ToLower(text)), " ")
	hash := sha256.Sum256([]byte(normalized))
	return hex.EncodeToString(hash[:])
}
