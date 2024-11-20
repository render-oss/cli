package text

import "fmt"

func FormatString(s string) string {
	return FormatStringF(s)
}

func FormatStringF(s string, a ...any) string {
	return fmt.Sprintf(s+"\n", a...)
}
