package main

import (
	"math"
	"strconv"
	"strings"

	"github.com/diamondburned/gotk4-adwaita/pkg/adw"
	"github.com/diamondburned/gotk4/pkg/gtk/v4"

	"github.com/zackb/minfin/internal/dashboard"
	"github.com/zackb/minfin/internal/store"
)

func (a *App) buildAccounts() gtk.Widgetter {
	if a.pid == "" {
		return emptyState()
	}
	accts, err := a.st.Accounts(a.pid, a.now())
	if err != nil {
		return errorState(err)
	}

	body := vbox(18)
	title := gtk.NewLabel("Accounts")
	title.AddCSSClass("title-1")
	title.SetXAlign(0)
	body.Append(title)

	grp := adw.NewPreferencesGroup()
	grp.SetDescription("Set a type so net worth and spending are accurate. Loan types take an underlying asset value (house, car).")
	for _, ac := range accts {
		grp.Add(a.accountRow(ac))
	}
	body.Append(grp)

	return pageBody(body)
}

// accountRow is an expander showing the balance, opening to type / nickname /
// asset-value editors that each write straight through the store and refresh.
func (a *App) accountRow(ac store.AccountInfo) *adw.ExpanderRow {
	row := expanderRow()
	row.SetTitle(ac.Display())
	subtitle := ac.Org
	if ac.LastTxn != "" {
		subtitle += " · last " + ac.LastTxn
	}
	row.SetSubtitle(subtitle)
	row.AddSuffix(moneySuffix(ac.Balance))

	// Type chooser. Index 0 is "Uncategorized" (empty key).
	typeRow := adw.NewComboRow()
	typeRow.SetTitle("Type")
	labels := []string{"Uncategorized"}
	keys := []string{""}
	for _, t := range store.AccountTypes {
		labels = append(labels, t.Label)
		keys = append(keys, t.Key)
	}
	typeRow.SetModel(gtk.NewStringList(labels))
	for i, k := range keys {
		if k == ac.Type {
			typeRow.SetSelected(uint(i))
		}
	}
	typeRow.Connect("notify::selected", func() {
		i := int(typeRow.Selected())
		if i < 0 || i >= len(keys) {
			return
		}
		if err := a.st.SetAccountType(a.pid, ac.ID, keys[i]); err != nil {
			a.toast("Couldn't set type: " + err.Error())
			return
		}
		a.refreshAll() // type can flip an account between asset/liability
	})
	row.AddRow(typeRow)

	// Nickname.
	nick := adw.NewEntryRow()
	nick.SetTitle("Nickname")
	nick.SetText(ac.Nickname)
	nick.ConnectApply(func() {
		if err := a.st.SetAccountNickname(a.pid, ac.ID, strings.TrimSpace(nick.Text())); err != nil {
			a.toast("Couldn't set nickname: " + err.Error())
			return
		}
		a.refreshAll()
	})
	row.AddRow(nick)

	// Asset value, only for loan types that carry one (mortgage, auto loan).
	if ac.HasAsset {
		asset := adw.NewEntryRow()
		asset.SetTitle("Asset value (e.g. house, car)")
		if ac.AssetValue != 0 {
			asset.SetText(dashboard.USD(ac.AssetValue))
		}
		asset.ConnectApply(func() {
			cents, err := parseDollars(asset.Text())
			if err != nil {
				a.toast("Enter a dollar amount, e.g. 250000")
				return
			}
			if err := a.st.SetAccountAssetValue(a.pid, ac.ID, cents); err != nil {
				a.toast("Couldn't set asset value: " + err.Error())
				return
			}
			a.refreshAll()
		})
		row.AddRow(asset)
	}

	return row
}

// parseDollars reads a user-entered dollar amount into cents, tolerating "$",
// commas and spaces. Empty clears (0 cents).
func parseDollars(s string) (int64, error) {
	s = strings.TrimSpace(s)
	s = strings.NewReplacer("$", "", ",", "", " ", "").Replace(s)
	if s == "" {
		return 0, nil
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, err
	}
	return int64(math.Round(f * 100)), nil
}
