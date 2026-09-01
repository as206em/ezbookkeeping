package services

import (
	"crypto/sha256"
	"encoding/hex"
	"strconv"
	"time"

	"xorm.io/xorm"

	"github.com/mayswind/ezbookkeeping/pkg/core"
	"github.com/mayswind/ezbookkeeping/pkg/datastore"
	"github.com/mayswind/ezbookkeeping/pkg/errs"
	"github.com/mayswind/ezbookkeeping/pkg/models"
	"github.com/mayswind/ezbookkeeping/pkg/uuid"
)

const bankMessageOutboxProcessingTimeout = 5 * time.Minute

// BankMessageOutboxService manages durable bank-message background work.
type BankMessageOutboxService struct {
	ServiceUsingDB
	ServiceUsingUuid
}

var BankMessageOutbox = &BankMessageOutboxService{
	ServiceUsingDB:   ServiceUsingDB{container: datastore.Container},
	ServiceUsingUuid: ServiceUsingUuid{container: uuid.Container},
}

func (s *BankMessageOutboxService) Enqueue(c core.Context, uid int64, message string, messageHash string) (*models.BankMessageOutbox, bool, error) {
	if uid <= 0 {
		return nil, false, errs.ErrUserIdInvalid
	}

	now := time.Now().Unix()
	existing, err := s.getByContentHash(c, uid, messageHash)
	if err != nil {
		return nil, false, err
	} else if existing != nil {
		return existing, true, nil
	}

	outboxId := s.GenerateUuid(uuid.UUID_TYPE_DEFAULT)
	reservedTransactionId := s.GenerateUuid(uuid.UUID_TYPE_TRANSACTION)
	item := &models.BankMessageOutbox{
		OutboxId:              outboxId,
		Uid:                   uid,
		MessageHash:           bankMessageScopedHash(messageHash, outboxId),
		ContentHash:           messageHash,
		ReservedTransactionId: reservedTransactionId,
		Message:               message,
		Status:                models.BANK_MESSAGE_OUTBOX_STATUS_QUEUED,
		CreatedUnixTime:       now,
		UpdatedUnixTime:       now,
	}

	if item.OutboxId <= 0 || item.ReservedTransactionId <= 0 {
		return nil, false, errs.ErrSystemIsBusy
	}

	idempotencyKey := &models.BankMessageIdempotencyKey{
		Uid:         uid,
		MessageHash: messageHash,
		OutboxId:    outboxId,
	}

	err = s.UserDB().DoTransaction(c, func(sess *xorm.Session) error {
		if _, err := sess.Insert(idempotencyKey); err != nil {
			return err
		}
		_, err := sess.Insert(item)
		return err
	})
	if err == nil {
		return item, false, nil
	}

	// A concurrent request may have inserted the same permanent idempotency key first.
	existing, getErr := s.getByContentHash(c, uid, messageHash)
	if getErr == nil && existing != nil {
		return existing, true, nil
	}

	return nil, false, err
}

func (s *BankMessageOutboxService) GetById(c core.Context, uid int64, outboxId int64) (*models.BankMessageOutbox, error) {
	item := &models.BankMessageOutbox{}
	has, err := s.UserDB().NewSession(c).Where("uid=? AND outbox_id=?", uid, outboxId).Get(item)
	if err != nil || !has {
		return nil, err
	}

	return item, nil
}

func (s *BankMessageOutboxService) getByContentHash(c core.Context, uid int64, messageHash string) (*models.BankMessageOutbox, error) {
	item := &models.BankMessageOutbox{}
	has, err := s.UserDB().NewSession(c).
		Where("uid=? AND content_hash=?", uid, messageHash).
		OrderBy("created_unix_time ASC").
		Get(item)
	if err != nil || !has {
		return nil, err
	}

	return item, nil
}

func (s *BankMessageOutboxService) ListByUid(c core.Context, uid int64, count int) ([]*models.BankMessageOutbox, error) {
	if uid <= 0 {
		return nil, errs.ErrUserIdInvalid
	}

	if count < 1 || count > 200 {
		count = 100
	}

	items := make([]*models.BankMessageOutbox, 0, count)
	err := s.UserDB().NewSession(c).Where("uid=?", uid).OrderBy("created_unix_time DESC").Limit(count).Find(&items)
	return items, err
}

func (s *BankMessageOutboxService) ClaimNext(c core.Context) (*models.BankMessageOutbox, error) {
	now := time.Now().Unix()
	staleBefore := now - int64(bankMessageOutboxProcessingTimeout/time.Second)

	for i := 0; i < 5; i++ {
		item := &models.BankMessageOutbox{}
		has, err := s.UserDB().NewSession(c).
			Where("(status=? OR (status=? AND next_retry_unix_time<=?) OR (status=? AND updated_unix_time<=?))",
				models.BANK_MESSAGE_OUTBOX_STATUS_QUEUED,
				models.BANK_MESSAGE_OUTBOX_STATUS_RETRYING, now,
				models.BANK_MESSAGE_OUTBOX_STATUS_PROCESSING, staleBefore).
			OrderBy("created_unix_time ASC").Limit(1).Get(item)
		if err != nil || !has {
			return nil, err
		}

		previousStatus := item.Status
		previousUpdatedTime := item.UpdatedUnixTime
		if item.ReservedTransactionId <= 0 {
			item.ReservedTransactionId = s.GenerateUuid(uuid.UUID_TYPE_TRANSACTION)
			if item.ReservedTransactionId <= 0 {
				return nil, errs.ErrSystemIsBusy
			}
		}
		item.Status = models.BANK_MESSAGE_OUTBOX_STATUS_PROCESSING
		item.UpdatedUnixTime = now
		item.NextRetryUnixTime = 0

		updated, err := s.UserDB().NewSession(c).Cols("reserved_transaction_id", "status", "updated_unix_time", "next_retry_unix_time").
			Where("outbox_id=? AND status=? AND updated_unix_time=?", item.OutboxId, previousStatus, previousUpdatedTime).
			Update(item)
		if err != nil {
			return nil, err
		} else if updated == 1 {
			return item, nil
		}
	}

	return nil, nil
}

func (s *BankMessageOutboxService) MarkSucceeded(c core.Context, item *models.BankMessageOutbox, transactionId int64) error {
	return s.markFinished(c, item, models.BANK_MESSAGE_OUTBOX_STATUS_SUCCEEDED, transactionId, "")
}

func (s *BankMessageOutboxService) MarkIgnored(c core.Context, item *models.BankMessageOutbox) error {
	return s.markFinished(c, item, models.BANK_MESSAGE_OUTBOX_STATUS_IGNORED, 0, "")
}

func (s *BankMessageOutboxService) MarkPaused(c core.Context, item *models.BankMessageOutbox) error {
	return s.markFinished(c, item, models.BANK_MESSAGE_OUTBOX_STATUS_PAUSED, 0, "Bank SMS automation is disabled")
}

func (s *BankMessageOutboxService) markFinished(c core.Context, item *models.BankMessageOutbox, status models.BankMessageOutboxStatus, transactionId int64, lastError string) error {
	update := &models.BankMessageOutbox{
		Status:          status,
		TransactionId:   transactionId,
		LastError:       lastError,
		UpdatedUnixTime: time.Now().Unix(),
	}
	_, err := s.UserDB().NewSession(c).Cols("status", "transaction_id", "last_error", "updated_unix_time").
		Where("outbox_id=? AND status=?", item.OutboxId, models.BANK_MESSAGE_OUTBOX_STATUS_PROCESSING).Update(update)
	return err
}

func bankMessageScopedHash(contentHash string, outboxId int64) string {
	hash := sha256.Sum256([]byte(contentHash + ":" + strconv.FormatInt(outboxId, 10)))
	return hex.EncodeToString(hash[:])
}

func (s *BankMessageOutboxService) MarkFailedAttempt(c core.Context, item *models.BankMessageOutbox, processingError error) error {
	now := time.Now().Unix()
	status := models.BANK_MESSAGE_OUTBOX_STATUS_RETRYING
	retryCount := item.RetryCount + 1
	nextRetryTime := now + int64(5*retryCount)

	if retryCount > 3 {
		status = models.BANK_MESSAGE_OUTBOX_STATUS_FAILED
		retryCount = 3
		nextRetryTime = 0
	}

	lastError := "Unknown processing error"
	if processingError != nil {
		lastError = processingError.Error()
	}

	update := &models.BankMessageOutbox{
		Status:            status,
		RetryCount:        retryCount,
		NextRetryUnixTime: nextRetryTime,
		LastError:         lastError,
		UpdatedUnixTime:   now,
	}
	_, err := s.UserDB().NewSession(c).Cols("status", "retry_count", "next_retry_unix_time", "last_error", "updated_unix_time").
		Where("outbox_id=? AND status=?", item.OutboxId, models.BANK_MESSAGE_OUTBOX_STATUS_PROCESSING).Update(update)
	return err
}
