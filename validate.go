package mintarex

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"
)

// Client-side validators that mirror the server regexes. Failing fast here
// saves a round-trip and gives a clearer error than a 400 from the API.

var (
	amountRE       = regexp.MustCompile(`^(?:0|[1-9]\d{0,29})(?:\.\d{1,18})?$`)
	addressTagRE   = regexp.MustCompile(`^[\x20-\x7E]{1,100}$`)
	coinRE         = regexp.MustCompile(`^[A-Z0-9]{2,10}$`)
	currencyFiatRE = regexp.MustCompile(`^[A-Z]{3,10}$`)
	currencyCodeRE = regexp.MustCompile(`^[A-Z0-9]{2,10}$`)
	networkRE      = regexp.MustCompile(`^[a-z0-9_-]{1,40}$`)
	addressRE      = regexp.MustCompile(`^[a-zA-Z0-9:._-]{10,255}$`)
	idempotencyRE  = regexp.MustCompile(`^[\x20-\x7E]{1,64}$`)
	labelRE        = regexp.MustCompile(`^[\x20-\x7E]{1,100}$`)
	uuidRE         = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[1-5][0-9a-fA-F]{3}-[89abAB][0-9a-fA-F]{3}-[0-9a-fA-F]{12}$`)
	eventRE        = regexp.MustCompile(`^[a-z]+\.[a-z_]+$`)
)

func reject(format string, args ...any) error {
	return &ValidationError{APIError: APIError{
		Status:  0,
		Code:    "client_validation",
		Message: fmt.Sprintf(format, args...),
	}}
}

func assertAmount(value, field string) (string, error) {
	if !amountRE.MatchString(value) {
		return "", reject("%s must be a decimal with <=30 integer digits and <=18 decimal places, no sign, no scientific notation", field)
	}
	return value, nil
}

func assertAddressTag(value, field string) (string, error) {
	if !addressTagRE.MatchString(value) {
		return "", reject("%s must be 1-100 printable ASCII characters", field)
	}
	return value, nil
}

func assertCoin(value, field string) (string, error) {
	if !coinRE.MatchString(value) {
		return "", reject("%s must be 2-10 uppercase letters or digits", field)
	}
	return value, nil
}

func assertFiatCurrency(value, field string) (string, error) {
	if !currencyFiatRE.MatchString(value) {
		return "", reject("%s must be 3-10 uppercase letters", field)
	}
	return value, nil
}

// AssertCurrencyCode accepts any currency code (fiat or crypto). Supports
// digit-leading codes like "1INCH" and "2Z". The server classifies the pair.
func assertCurrencyCode(value, field string) (string, error) {
	if !currencyCodeRE.MatchString(value) {
		return "", reject("%s must be 2-10 uppercase letters or digits", field)
	}
	return value, nil
}

func assertNetwork(value, field string) (string, error) {
	if !networkRE.MatchString(value) {
		return "", reject("%s must be 1-40 lowercase [a-z0-9_-]", field)
	}
	return value, nil
}

func assertAddress(value, field string) (string, error) {
	if !addressRE.MatchString(value) {
		return "", reject("%s must be 10-255 chars, alphanumeric + : . _ -", field)
	}
	return value, nil
}

func assertIdempotencyKey(value, field string) (string, error) {
	if !idempotencyRE.MatchString(value) {
		return "", reject("%s must be 1-64 printable ASCII characters", field)
	}
	return value, nil
}

func assertLabel(value, field string) (string, error) {
	if !labelRE.MatchString(value) {
		return "", reject("%s must be 1-100 printable ASCII characters", field)
	}
	return value, nil
}

func assertSide(value, field string) (string, error) {
	if value != "buy" && value != "sell" {
		return "", reject("%s must be \"buy\" or \"sell\"", field)
	}
	return value, nil
}

func assertAmountType(value, field string) (string, error) {
	if value != "base" && value != "quote" {
		return "", reject("%s must be \"base\" or \"quote\"", field)
	}
	return value, nil
}

func assertUUID(value, field string) (string, error) {
	if !uuidRE.MatchString(value) {
		return "", reject("%s must be a valid UUID", field)
	}
	return strings.ToLower(value), nil
}

func assertHTTPSURL(value, field string) (string, error) {
	if len(value) > 2048 {
		return "", reject("%s too long (max 2048)", field)
	}
	u, err := url.Parse(value)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return "", reject("%s is not a valid URL", field)
	}
	if u.Scheme != "https" {
		return "", reject("%s must use https://", field)
	}
	if u.User != nil {
		return "", reject("%s must not contain credentials", field)
	}
	return value, nil
}

func assertEvents(value []string, field string) ([]string, error) {
	if len(value) == 0 {
		return nil, reject("%s must be a non-empty array", field)
	}
	out := make([]string, 0, len(value))
	seen := make(map[string]struct{}, len(value))
	for _, ev := range value {
		if !eventRE.MatchString(ev) {
			return nil, reject("%s entries must look like \"domain.action\" (lowercase)", field)
		}
		if _, ok := seen[ev]; ok {
			continue
		}
		seen[ev] = struct{}{}
		out = append(out, ev)
	}
	return out, nil
}
