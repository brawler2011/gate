// Command judgeimage prints the go-judge image name for the current judge/ sources.
package main

import (
	"fmt"
	"os"

	"github.com/brawler2011/gate/backend/tests/judgeimage"
)

func main() {
	ref, err := judgeimage.Ref()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Println(ref)
}
