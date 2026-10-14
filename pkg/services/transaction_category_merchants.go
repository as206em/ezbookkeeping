package services

import (
	"time"

	"xorm.io/xorm"

	"github.com/mayswind/ezbookkeeping/pkg/core"
	"github.com/mayswind/ezbookkeeping/pkg/datastore"
	"github.com/mayswind/ezbookkeeping/pkg/errs"
	"github.com/mayswind/ezbookkeeping/pkg/models"
)

// TransactionCategoryMerchantService represents transaction category merchant rule service
type TransactionCategoryMerchantService struct {
	ServiceUsingDB
}

// Initialize a transaction category merchant rule service singleton instance
var (
	TransactionCategoryMerchants = &TransactionCategoryMerchantService{
		ServiceUsingDB: ServiceUsingDB{
			container: datastore.Container,
		},
	}
)

// GetAllMerchantsByUid returns all merchant rules of a user, ordered by merchant name
func (s *TransactionCategoryMerchantService) GetAllMerchantsByUid(c core.Context, uid int64) ([]*models.TransactionCategoryMerchant, error) {
	if uid <= 0 {
		return nil, errs.ErrUserIdInvalid
	}

	var merchants []*models.TransactionCategoryMerchant
	err := s.UserDataDB(uid).NewSession(c).Where("uid=?", uid).OrderBy("merchant_key asc").Find(&merchants)

	return merchants, err
}

// SetMerchantCategory maps a merchant to a category, moving it from any category it was mapped to before
func (s *TransactionCategoryMerchantService) SetMerchantCategory(c core.Context, uid int64, merchant string, categoryId int64) (*models.TransactionCategoryMerchant, error) {
	if uid <= 0 {
		return nil, errs.ErrUserIdInvalid
	}

	merchant = models.NormalizeMerchantName(merchant)
	merchantKey := models.GetMerchantKey(merchant)

	if merchantKey == "" || len([]rune(merchant)) > models.TransactionCategoryMerchantMaxLength {
		return nil, errs.ErrTransactionCategoryMerchantInvalid
	}

	now := time.Now().Unix()
	rule := &models.TransactionCategoryMerchant{
		Uid:             uid,
		MerchantKey:     merchantKey,
		Merchant:        merchant,
		CategoryId:      categoryId,
		CreatedUnixTime: now,
		UpdatedUnixTime: now,
	}

	err := s.UserDataDB(uid).DoTransaction(c, func(sess *xorm.Session) error {
		exists, err := sess.Where("uid=? AND merchant_key=?", uid, merchantKey).Exist(&models.TransactionCategoryMerchant{})

		if err != nil {
			return err
		}

		if exists {
			_, err = sess.Cols("merchant", "category_id", "updated_unix_time").Where("uid=? AND merchant_key=?", uid, merchantKey).Update(rule)
		} else {
			_, err = sess.Insert(rule)
		}

		return err
	})

	if err != nil {
		return nil, err
	}

	return rule, nil
}

// DeleteMerchant removes the category mapping of a merchant
func (s *TransactionCategoryMerchantService) DeleteMerchant(c core.Context, uid int64, merchant string) error {
	if uid <= 0 {
		return errs.ErrUserIdInvalid
	}

	return s.UserDataDB(uid).DoTransaction(c, func(sess *xorm.Session) error {
		_, err := sess.Where("uid=? AND merchant_key=?", uid, models.GetMerchantKey(merchant)).Delete(&models.TransactionCategoryMerchant{})
		return err
	})
}

// DeleteAllMerchants deletes all merchant rules of a user
func (s *TransactionCategoryMerchantService) DeleteAllMerchants(c core.Context, uid int64) error {
	if uid <= 0 {
		return errs.ErrUserIdInvalid
	}

	return s.UserDataDB(uid).DoTransaction(c, func(sess *xorm.Session) error {
		_, err := sess.Where("uid=?", uid).Delete(&models.TransactionCategoryMerchant{})
		return err
	})
}
