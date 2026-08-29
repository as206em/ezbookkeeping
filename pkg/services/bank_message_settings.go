package services

import (
	"time"

	"xorm.io/xorm"

	"github.com/mayswind/ezbookkeeping/pkg/core"
	"github.com/mayswind/ezbookkeeping/pkg/datastore"
	"github.com/mayswind/ezbookkeeping/pkg/errs"
	"github.com/mayswind/ezbookkeeping/pkg/models"
)

const bankMessageEnabledUserSlot int8 = 1

// BankMessageSettingsService manages per-user bank-message automation settings.
type BankMessageSettingsService struct {
	ServiceUsingDB
}

var BankMessageSettings = &BankMessageSettingsService{
	ServiceUsingDB: ServiceUsingDB{
		container: datastore.Container,
	},
}

func (s *BankMessageSettingsService) GetByUid(c core.Context, uid int64) (*models.BankMessageAutomationSetting, error) {
	if uid <= 0 {
		return nil, errs.ErrUserIdInvalid
	}

	setting := &models.BankMessageAutomationSetting{}
	has, err := s.UserDB().NewSession(c).ID(uid).Get(setting)

	if err != nil {
		return nil, err
	} else if !has {
		return nil, nil
	}

	return setting, nil
}

func (s *BankMessageSettingsService) GetEnabled(c core.Context) (*models.BankMessageAutomationSetting, error) {
	settings := make([]*models.BankMessageAutomationSetting, 0, 2)
	err := s.UserDB().NewSession(c).Where("enabled=?", true).Limit(2).Find(&settings)

	if err != nil {
		return nil, err
	} else if len(settings) == 0 {
		return nil, errs.ErrBankMessageAutomationNotEnabled
	} else if len(settings) > 1 {
		return nil, errs.ErrBankMessageAutomationAmbiguous
	}

	return settings[0], nil
}

func (s *BankMessageSettingsService) Update(c core.Context, setting *models.BankMessageAutomationSetting) error {
	if setting == nil || setting.Uid <= 0 {
		return errs.ErrUserIdInvalid
	}

	setting.UpdatedUnixTime = time.Now().Unix()
	attempts := 1
	if setting.Enabled {
		attempts = 10
	}

	var err error
	for attempt := 0; attempt < attempts; attempt++ {
		err = s.updateInTransaction(c, setting)
		if err == nil || !setting.Enabled {
			return err
		}

		guard := &models.BankMessageEnabledUser{}
		has, guardErr := s.UserDB().NewSession(c).ID(bankMessageEnabledUserSlot).Get(guard)
		if guardErr == nil && has && guard.Uid != setting.Uid {
			return errs.ErrBankMessageAlreadyEnabled
		}

		if err == errs.ErrBankMessageAlreadyEnabled {
			return err
		}

		if attempt+1 < attempts {
			time.Sleep(time.Duration(attempt+1) * 20 * time.Millisecond)
		}
	}

	guard := &models.BankMessageEnabledUser{}
	has, guardErr := s.UserDB().NewSession(c).ID(bankMessageEnabledUserSlot).Get(guard)
	if guardErr == nil && has && guard.Uid != setting.Uid {
		return errs.ErrBankMessageAlreadyEnabled
	}

	return err
}

func (s *BankMessageSettingsService) updateInTransaction(c core.Context, setting *models.BankMessageAutomationSetting) error {
	return s.UserDB().DoTransaction(c, func(sess *xorm.Session) error {
		if setting.Enabled {
			count, err := sess.Where("enabled=? AND uid<>?", true, setting.Uid).Count(&models.BankMessageAutomationSetting{})

			if err != nil {
				return err
			} else if count > 0 {
				return errs.ErrBankMessageAlreadyEnabled
			}

			guard := &models.BankMessageEnabledUser{}
			has, err := sess.ID(bankMessageEnabledUserSlot).Get(guard)
			if err != nil {
				return err
			} else if has && guard.Uid != setting.Uid {
				return errs.ErrBankMessageAlreadyEnabled
			} else if !has {
				if _, err = sess.Insert(&models.BankMessageEnabledUser{Slot: bankMessageEnabledUserSlot, Uid: setting.Uid}); err != nil {
					return err
				}
			}
		} else {
			if _, err := sess.Where("slot=? AND uid=?", bankMessageEnabledUserSlot, setting.Uid).Delete(&models.BankMessageEnabledUser{}); err != nil {
				return err
			}
		}

		exists, err := sess.ID(setting.Uid).Exist(&models.BankMessageAutomationSetting{})

		if err != nil {
			return err
		}

		if !exists {
			_, err = sess.Insert(setting)
		} else {
			_, err = sess.ID(setting.Uid).Cols("enabled", "prompt", "account_mappings", "updated_unix_time").Update(setting)
		}

		if err != nil {
			return err
		}

		if setting.Enabled {
			resume := &models.BankMessageOutbox{
				Status:          models.BANK_MESSAGE_OUTBOX_STATUS_QUEUED,
				LastError:       "",
				UpdatedUnixTime: setting.UpdatedUnixTime,
			}
			_, err = sess.Cols("status", "last_error", "updated_unix_time").
				Where("uid=? AND status=?", setting.Uid, models.BANK_MESSAGE_OUTBOX_STATUS_PAUSED).Update(resume)
		}

		return err
	})
}

func (s *BankMessageSettingsService) DeleteByUid(c core.Context, uid int64) error {
	if uid <= 0 {
		return errs.ErrUserIdInvalid
	}

	return s.UserDB().DoTransaction(c, func(sess *xorm.Session) error {
		if _, err := sess.Where("slot=? AND uid=?", bankMessageEnabledUserSlot, uid).Delete(&models.BankMessageEnabledUser{}); err != nil {
			return err
		}
		_, err := sess.ID(uid).Delete(&models.BankMessageAutomationSetting{})
		return err
	})
}
