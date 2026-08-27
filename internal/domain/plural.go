package domain

import "fmt"

// Ukrainian picks a form by the last digit, with the teens as an exception:
// 1 серія, 2 серії, 5 серій, 21 серія, but 11 серій.
func Plural(n int, one, few, many string) string {
	if n < 0 {
		n = -n
	}
	switch {
	case n%100 >= 11 && n%100 <= 14:
		return many
	case n%10 == 1:
		return one
	case n%10 >= 2 && n%10 <= 4:
		return few
	default:
		return many
	}
}

func Count(n int, one, few, many string) string {
	return fmt.Sprintf("%d %s", n, Plural(n, one, few, many))
}
