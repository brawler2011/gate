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
	
	if n <= 0 {
		return
	}
	
	scanner.Scan()
	val, _ := strconv.ParseInt(scanner.Text(), 10, 64)
	maxSoFar := val
	currMax := val
	
	for i := 1; i < n; i++ {
		if !scanner.Scan() {
			break
		}
		val, _ = strconv.ParseInt(scanner.Text(), 10, 64)
		if val > currMax + val {
			currMax = val
		} else {
			currMax = currMax + val
		}
		if currMax > maxSoFar {
			maxSoFar = currMax
		}
	}
	fmt.Println(maxSoFar)
}
