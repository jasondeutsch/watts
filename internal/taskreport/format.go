package taskreport

import (
	"strconv"
	"strings"
)

func integerText(value int64) string {
	digits := strconv.FormatInt(value, 10)
	sign := ""
	if strings.HasPrefix(digits, "-") {
		sign, digits = "-", digits[1:]
	}
	groups := []string{}
	for len(digits) > 3 {
		groups = append([]string{digits[len(digits)-3:]}, groups...)
		digits = digits[:len(digits)-3]
	}
	return sign + strings.Join(append([]string{digits}, groups...), ",")
}
