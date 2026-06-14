package models

import (
	"sort"
	"strings"
	"time"

	log "github.com/rdumanski/gophish/logger"
)

// OrgGroupSep joins the segments of an auto-group's hierarchical name, e.g.
// "Departament Eksploatacji / Biuro Ruchu / Wydzial Nadzoru". The full path is
// used so a sub-unit name that repeats under different parents stays unique.
const OrgGroupSep = " / "

// RegenerateOrgGroups rebuilds the operator's system-managed (is_auto) groups,
// one per distinct org unit (Department, then Department/Sub-Department, then
// Department/Sub-Department/Wydzial) found across their Recipients. A recipient
// belongs to its department group, its sub-department group, and its wydzial
// group, so a campaign can target any level and reach the right people.
//
// It's a full rebuild (drop existing auto-groups, recreate), so it's safe to
// call repeatedly — after a roster sync or on demand. Returns the number of
// auto-groups created. Recipients without a department or without an email are
// skipped (no targetable unit / no contact key).
func RegenerateOrgGroups(uid int64) (int, error) {
	recipients := []Recipient{}
	if err := db.Where("user_id = ?", uid).Find(&recipients).Error; err != nil {
		log.Error(err)
		return 0, err
	}

	// Bucket recipients by org-unit path.
	buckets := map[string][]BaseRecipient{}
	addMember := func(name string, br BaseRecipient) {
		buckets[name] = append(buckets[name], br)
	}
	for _, rc := range recipients {
		if strings.TrimSpace(rc.Email) == "" {
			continue
		}
		dept := strings.TrimSpace(rc.Department)
		if dept == "" {
			continue
		}
		sub := strings.TrimSpace(rc.SubDepartment)
		wyd := strings.TrimSpace(rc.Wydzial)
		br := BaseRecipient{
			Email: rc.Email, FirstName: rc.FirstName, LastName: rc.LastName,
			Position: rc.Position, Phone: rc.Phone,
			Department: rc.Department, SubDepartment: rc.SubDepartment,
			Wydzial: rc.Wydzial, PositionLevel: rc.PositionLevel,
		}
		addMember(dept, br)
		if sub != "" {
			addMember(dept+OrgGroupSep+sub, br)
			if wyd != "" {
				addMember(dept+OrgGroupSep+sub+OrgGroupSep+wyd, br)
			}
		}
	}

	names := make([]string, 0, len(buckets))
	for n := range buckets {
		names = append(names, n)
	}
	sort.Strings(names)

	tx := db.Begin()
	// Drop existing auto-groups and their membership links.
	autoIDs := []int64{}
	if err := tx.Model(&Group{}).Where("user_id = ? AND is_auto = ?", uid, true).
		Pluck("id", &autoIDs).Error; err != nil {
		tx.Rollback()
		log.Error(err)
		return 0, err
	}
	if len(autoIDs) > 0 {
		if err := tx.Where("group_id IN ?", autoIDs).Delete(&GroupTarget{}).Error; err != nil {
			tx.Rollback()
			return 0, err
		}
		if err := tx.Where("id IN ?", autoIDs).Delete(&Group{}).Error; err != nil {
			tx.Rollback()
			return 0, err
		}
	}

	created := 0
	for _, name := range names {
		g := Group{Name: name, UserID: uid, IsAuto: true, ModifiedDate: time.Now().UTC()}
		if err := tx.Save(&g).Error; err != nil {
			tx.Rollback()
			log.Error(err)
			return 0, err
		}
		for _, br := range buckets[name] {
			if err := insertTargetIntoGroup(tx, Target{BaseRecipient: br}, g.Id); err != nil {
				tx.Rollback()
				return 0, err
			}
		}
		created++
	}
	if err := tx.Commit().Error; err != nil {
		log.Error(err)
		return 0, err
	}
	return created, nil
}
