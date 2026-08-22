package main

import (
	"fmt"
)

func main() {
	var n int
	if _, err := fmt.Scan(&n); err == nil {
		fmt.Println("! 1")
	}
}
