package store

// AccountType is a user-assignable account category. Liability marks the ones
// that count against net worth (their balances already arrive negative).
type AccountType struct {
	Key       string
	Label     string
	Liability bool
}

var AccountTypes = []AccountType{
	{"checking", "Checking", false},
	{"savings", "Savings", false},
	{"investment", "Investment", false},
	{"credit_card", "Credit Card", true},
	{"mortgage", "Mortgage", true},
	{"auto_loan", "Auto Loan", true},
	{"loan", "Loan", true},
	{"other", "Other", false},
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
