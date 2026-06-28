package store

// AccountType is a user-assignable account category. Liability marks the ones
// that count against net worth (their balances already arrive negative). Asset
// marks loan types with an underlying asset (house, car) whose value the user
// can enter so net worth and equity are correct.
type AccountType struct {
	Key       string
	Label     string
	Liability bool
	Asset     bool
}

var AccountTypes = []AccountType{
	{"checking", "Checking", false, false},
	{"savings", "Savings", false, false},
	{"investment", "Investment", false, false},
	{"credit_card", "Credit Card", true, false},
	{"mortgage", "Mortgage", true, true},
	{"auto_loan", "Auto Loan", true, true},
	{"loan", "Loan", true, false},
	{"other", "Other", false, false},
}

// HasAsset reports whether a type carries an underlying asset value.
func HasAsset(typ string) bool {
	for _, t := range AccountTypes {
		if t.Key == typ {
			return t.Asset
		}
	}
	return false
}

// ValidType reports whether key is the empty (uncategorized) value or a known type.
func ValidType(key string) bool {
	if key == "" {
		return true
	}
	for _, t := range AccountTypes {
		if t.Key == key {
			return true
		}
	}
	return false
}

// Classify reports whether an account is a liability. The type wins; an
// uncategorized account falls back to its balance sign so the Assets/Liabilities
// split is right before anything is tagged.
// ponytail: sign fallback until user tags.
func Classify(typ string, balanceCents int64) bool {
	for _, t := range AccountTypes {
		if t.Key == typ {
			return t.Liability
		}
	}
	return balanceCents < 0
}
