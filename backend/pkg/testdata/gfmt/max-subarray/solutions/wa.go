package main

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
)

func main() {
	scanner := bufio.NewScanner(os.Stdin)
	scanner.Split(bufio.ScanWords)
	
	if !scanner.Scan() {
		return
	}
	n, _ := strconv.Atoi(scanner.Text())
	
	maxSoFar := int64(0)
	currMax := int64(0)
	
	for i := 0; i < n; i++ {
		if !scanner.Scan() {
			break
		}
		val, _ := strconv.ParseInt(scanner.Text(), 10, 64)
		if currMax + val > 0 {
			currMax = currMax + val
		} else {
			currMax = 0
		}
		if currMax > maxSoFar {
			maxSoFar = currMax
		}
	}
	fmt.Println(maxSoFar)
}
