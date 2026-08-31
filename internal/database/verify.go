package database

import (
	"database/sql"
	"errors"
	"time"

	"github.com/go-jet/jet/v2/qrm"
	. "github.com/go-jet/jet/v2/sqlite"
	"goisekai/internal/database/.gen/model"
	tbl "goisekai/internal/database/.gen/table"
)

// PluginVerify holds a plugin's human-verification state: the URL the user
// opens to solve the challenge and the cookies/User-Agent pasted back after
// solving. It lives in its own table because the values are credentials with a
// paste/overwrite/expire lifecycle separate from plugin metadata.
type PluginVerifyRow struct {
	PluginID  string
	VerifyURL string
	Cookies   string
	UserAgent string
	UpdatedAt int64
}

// UpsertPluginVerify inserts a plugin's verification data or overwrites it on a
// duplicate plugin id, stamping updated_at with the current unix time.
func (d *DB) UpsertPluginVerify(v PluginVerifyRow) error {
	now := time.Now().Unix()
	_, err := tbl.PluginVerify.INSERT(
		tbl.PluginVerify.PluginID,
		tbl.PluginVerify.VerifyURL,
		tbl.PluginVerify.Cookies,
		tbl.PluginVerify.UserAgent,
		tbl.PluginVerify.UpdatedAt,
	).VALUES(
		v.PluginID,
		v.VerifyURL,
		v.Cookies,
		v.UserAgent,
		now,
	).ON_CONFLICT(tbl.PluginVerify.PluginID).DO_UPDATE(
		SET(
			tbl.PluginVerify.VerifyURL.SET(tbl.PluginVerify.EXCLUDED.VerifyURL),
			tbl.PluginVerify.Cookies.SET(tbl.PluginVerify.EXCLUDED.Cookies),
			tbl.PluginVerify.UserAgent.SET(tbl.PluginVerify.EXCLUDED.UserAgent),
			tbl.PluginVerify.UpdatedAt.SET(tbl.PluginVerify.EXCLUDED.UpdatedAt),
		),
	).Exec(d.db)
	return err
}

// GetPluginVerify returns the stored verification data for a plugin. The bool
// reports whether a row exists.
func (d *DB) GetPluginVerify(pluginID string) (PluginVerifyRow, bool, error) {
	var m model.PluginVerify
	err := tbl.PluginVerify.SELECT(tbl.PluginVerify.AllColumns).
		WHERE(tbl.PluginVerify.PluginID.EQ(String(pluginID))).
		Query(d.db, &m)
	if err == sql.ErrNoRows || errors.Is(err, qrm.ErrNoRows) {
		return PluginVerifyRow{}, false, nil
	}
	if err != nil {
		return PluginVerifyRow{}, false, err
	}
	return PluginVerifyRow{
		PluginID:  derefStr(m.PluginID),
		VerifyURL: m.VerifyURL,
		Cookies:   m.Cookies,
		UserAgent: m.UserAgent,
		UpdatedAt: m.UpdatedAt,
	}, true, nil
}
