package services

import (
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mayswind/ezbookkeeping/pkg/core"
	"github.com/mayswind/ezbookkeeping/pkg/datastore"
	"github.com/mayswind/ezbookkeeping/pkg/errs"
	"github.com/mayswind/ezbookkeeping/pkg/models"
	"github.com/mayswind/ezbookkeeping/pkg/settings"
	"github.com/mayswind/ezbookkeeping/pkg/uuid"
)

func initializeBankMessageServiceTest(t *testing.T) core.Context {
	t.Helper()

	config := &settings.Config{
		DatabaseConfig: &settings.DatabaseConfig{
			DatabaseType:          settings.Sqlite3DbType,
			DatabasePath:          filepath.Join(t.TempDir(), "bank-message-test.db"),
			MaxIdleConnection:     4,
			MaxOpenConnection:     4,
			ConnectionMaxLifeTime: 60,
		},
		UuidGeneratorType: settings.InternalUuidGeneratorType,
		UuidServerId:      1,
	}

	require.NoError(t, datastore.InitializeDataStore(config))
	require.NoError(t, uuid.InitializeUuidGenerator(config))
	t.Cleanup(func() {
		require.NoError(t, datastore.Container.UserStore.Choose(0).Close())
	})
	require.NoError(t, datastore.Container.UserStore.SyncStructs(
		new(models.BankMessageAutomationSetting),
		new(models.BankMessageEnabledUser),
		new(models.BankMessageOutbox),
		new(models.BankMessageIdempotencyKey),
	))
	require.NoError(t, datastore.Container.UserDataStore.SyncStructs(new(models.Transaction)))

	return core.NewNullContext()
}

func TestBankMessageOutboxDeduplicatesPermanently(t *testing.T) {
	c := initializeBankMessageServiceTest(t)
	messageHash := "1111111111111111111111111111111111111111111111111111111111111111"

	first, duplicate, err := BankMessageOutbox.Enqueue(c, 1, "same bank message", messageHash)
	require.NoError(t, err)
	assert.False(t, duplicate)
	assert.Positive(t, first.ReservedTransactionId)

	second, duplicate, err := BankMessageOutbox.Enqueue(c, 1, "same bank message", messageHash)
	require.NoError(t, err)
	assert.True(t, duplicate)
	assert.Equal(t, first.OutboxId, second.OutboxId)

	_, err = BankMessageOutbox.UserDB().NewSession(c).
		Where("uid=? AND message_hash=?", 1, messageHash).
		Delete(&models.BankMessageIdempotencyKey{})
	require.NoError(t, err)

	third, duplicate, err := BankMessageOutbox.Enqueue(c, 1, "same bank message", messageHash)
	require.NoError(t, err)
	assert.True(t, duplicate)
	assert.Equal(t, first.OutboxId, third.OutboxId)
}

func TestBankMessageOutboxClaimReservesTransactionIdForLegacyRows(t *testing.T) {
	c := initializeBankMessageServiceTest(t)
	now := time.Now().Unix()
	legacy := &models.BankMessageOutbox{
		OutboxId:        100,
		Uid:             1,
		MessageHash:     "2222222222222222222222222222222222222222222222222222222222222222",
		Message:         "legacy queued message",
		Status:          models.BANK_MESSAGE_OUTBOX_STATUS_QUEUED,
		CreatedUnixTime: now,
		UpdatedUnixTime: now,
	}
	_, err := BankMessageOutbox.UserDB().NewSession(c).Insert(legacy)
	require.NoError(t, err)

	claimed, err := BankMessageOutbox.ClaimNext(c)
	require.NoError(t, err)
	require.NotNil(t, claimed)
	assert.Positive(t, claimed.ReservedTransactionId)

	persisted, err := BankMessageOutbox.GetById(c, 1, legacy.OutboxId)
	require.NoError(t, err)
	assert.Equal(t, claimed.ReservedTransactionId, persisted.ReservedTransactionId)
}

func TestCreateTransactionWithIdReusesExistingTransaction(t *testing.T) {
	c := initializeBankMessageServiceTest(t)
	existing := &models.Transaction{
		TransactionId:   300,
		Uid:             1,
		Type:            models.TRANSACTION_DB_TYPE_EXPENSE,
		CategoryId:      10,
		AccountId:       20,
		TransactionTime: 30,
		Amount:          400,
		Comment:         "already created",
	}
	_, err := Transactions.UserDataDB(existing.Uid).NewSession(c).Insert(existing)
	require.NoError(t, err)

	retry := &models.Transaction{Uid: existing.Uid}
	require.NoError(t, Transactions.CreateTransactionWithId(c, retry, nil, nil, existing.TransactionId))
	assert.Equal(t, existing.TransactionId, retry.TransactionId)
	assert.Equal(t, existing.Comment, retry.Comment)

	count, err := Transactions.UserDataDB(existing.Uid).NewSession(c).
		Where("uid=? AND transaction_id=?", existing.Uid, existing.TransactionId).
		Count(&models.Transaction{})
	require.NoError(t, err)
	assert.Equal(t, int64(1), count)
}

func TestBankMessageSettingsEnforcesSingleEnabledUser(t *testing.T) {
	c := initializeBankMessageServiceTest(t)
	settingsToEnable := []*models.BankMessageAutomationSetting{
		{Uid: 1, Enabled: true, Prompt: models.DefaultBankMessagePrompt},
		{Uid: 2, Enabled: true, Prompt: models.DefaultBankMessagePrompt},
	}

	start := make(chan struct{})
	results := make(chan error, len(settingsToEnable))
	var waitGroup sync.WaitGroup
	for _, setting := range settingsToEnable {
		waitGroup.Add(1)
		go func(setting *models.BankMessageAutomationSetting) {
			defer waitGroup.Done()
			<-start
			results <- BankMessageSettings.Update(c, setting)
		}(setting)
	}
	close(start)
	waitGroup.Wait()
	close(results)

	successes := 0
	for err := range results {
		if err == nil {
			successes++
		} else {
			assert.Equal(t, errs.ErrBankMessageAlreadyEnabled, err)
		}
	}
	assert.Equal(t, 1, successes)

	enabledCount, err := BankMessageSettings.UserDB().NewSession(c).
		Where("enabled=?", true).Count(&models.BankMessageAutomationSetting{})
	require.NoError(t, err)
	assert.Equal(t, int64(1), enabledCount)

	guards := make([]*models.BankMessageEnabledUser, 0, 2)
	require.NoError(t, BankMessageSettings.UserDB().NewSession(c).Find(&guards))
	require.Len(t, guards, 1)
	assert.Equal(t, bankMessageEnabledUserSlot, guards[0].Slot)
}

func TestBankMessageSettingsPausesUntilReenabled(t *testing.T) {
	c := initializeBankMessageServiceTest(t)
	now := time.Now().Unix()
	item := &models.BankMessageOutbox{
		OutboxId:              200,
		Uid:                   1,
		MessageHash:           "3333333333333333333333333333333333333333333333333333333333333333",
		ReservedTransactionId: 201,
		Message:               "paused message",
		Status:                models.BANK_MESSAGE_OUTBOX_STATUS_PAUSED,
		LastError:             "Bank SMS automation is disabled",
		CreatedUnixTime:       now,
		UpdatedUnixTime:       now,
	}
	_, err := BankMessageOutbox.UserDB().NewSession(c).Insert(item)
	require.NoError(t, err)

	require.NoError(t, BankMessageSettings.Update(c, &models.BankMessageAutomationSetting{
		Uid: 1, Enabled: true, Prompt: models.DefaultBankMessagePrompt,
	}))

	resumed, err := BankMessageOutbox.GetById(c, 1, item.OutboxId)
	require.NoError(t, err)
	assert.Equal(t, models.BANK_MESSAGE_OUTBOX_STATUS_QUEUED, resumed.Status)
	assert.Empty(t, resumed.LastError)

	err = BankMessageSettings.Update(c, &models.BankMessageAutomationSetting{
		Uid: 2, Enabled: true, Prompt: models.DefaultBankMessagePrompt,
	})
	assert.Equal(t, errs.ErrBankMessageAlreadyEnabled, err)
}
