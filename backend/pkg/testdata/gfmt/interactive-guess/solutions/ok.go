package main

import (
	"fmt"
)

func main() {
	var n int
	if _, err := fmt.Scan(&n); err != nil {
		return
	}
	low, high := 1, n
	for low <= high {
		mid := (low + high) / 2
		fmt.Printf("? %d\n", mid)
		var response string
		if _, err := fmt.Scan(&response); err != nil {
			break
		}
		if response == "=" {
			fmt.Printf("! %d\n", mid)
			break
		} else if response == ">" {
			low = mid + 1
		} else {
			high = mid - 1
		}
	}
}
