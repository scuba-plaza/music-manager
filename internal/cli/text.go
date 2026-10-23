package cli

import "fmt"

// plural writes a count with the word for it: "1 track", "3 tracks".
func plural(n int, one, many string) string {
	if n == 1 {
		return fmt.Sprintf("%d %s", n, one)
	}
	return fmt.Sprintf("%d %s", n, many)
}
