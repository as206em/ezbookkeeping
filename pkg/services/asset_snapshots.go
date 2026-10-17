package services

import (
	"time"

	"xorm.io/xorm"

	"github.com/mayswind/ezbookkeeping/pkg/core"
	"github.com/mayswind/ezbookkeeping/pkg/errs"
	"github.com/mayswind/ezbookkeeping/pkg/log"
	"github.com/mayswind/ezbookkeeping/pkg/models"
)

// GetSnapshotDate returns the snapshot day (yyyymmdd, UTC) of a time
func GetSnapshotDate(t time.Time) int32 {
	utc := t.UTC()

	return int32(utc.Year()*10000 + int(utc.Month())*100 + utc.Day())
}

// SaveSnapshot records what the holdings of a user are worth today, replacing the earlier record of today.
// When nothing is held (or priced) today's record is removed, so the history shows the holdings really going to zero.
func (s *AssetService) SaveSnapshot(c core.Context, uid int64) error {
	if uid <= 0 {
		return errs.ErrUserIdInvalid
	}

	_, positions, err := s.GetPositionsByUid(c, uid)

	if err != nil {
		return err
	}

	now := time.Now()
	date := GetSnapshotDate(now)
	snapshots := models.BuildAssetSnapshots(uid, date, positions.Holdings, now.Unix())

	return s.UserDataDB(uid).DoTransaction(c, func(sess *xorm.Session) error {
		_, err := sess.Where("uid=? AND snapshot_date=?", uid, date).Delete(&models.AssetSnapshot{})

		if err != nil {
			return err
		}

		if len(snapshots) == 0 {
			return nil
		}

		_, err = sess.Insert(snapshots)

		return err
	})
}

// saveSnapshotQuietly keeps today's snapshot current after a change. A failure is logged and never fails the change itself.
func (s *AssetService) saveSnapshotQuietly(c core.Context, uid int64) {
	if err := s.SaveSnapshot(c, uid); err != nil {
		log.Errorf(c, "[asset_snapshots.saveSnapshotQuietly] failed to save snapshot for user \"uid:%d\", because %s", uid, err.Error())
	}
}

// GetSnapshotsByUid returns the most recent snapshots of a user, oldest day first
func (s *AssetService) GetSnapshotsByUid(c core.Context, uid int64) ([]*models.AssetSnapshot, error) {
	if uid <= 0 {
		return nil, errs.ErrUserIdInvalid
	}

	var snapshots []*models.AssetSnapshot
	err := s.UserDataDB(uid).NewSession(c).Where("uid=?", uid).OrderBy("snapshot_date desc, market asc, currency asc").Limit(models.AssetSnapshotMaxRows).Find(&snapshots)

	if err != nil {
		return nil, err
	}

	for i, j := 0, len(snapshots)-1; i < j; i, j = i+1, j-1 {
		snapshots[i], snapshots[j] = snapshots[j], snapshots[i]
	}

	return snapshots, nil
}
