// Command judgeimage prints the image name for the current judge/ sources, or typeset/ ones with the argument typeset.
package main

import (
	"fmt"
	"os"

	"github.com/brawler2011/gate/backend/tests/judgeimage"
)

func main() {
	ref := judgeimage.Ref
	if len(os.Args) > 1 && os.Args[1] == "typeset" {
		ref = judgeimage.TypesetRef
	}
	name, err := ref()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Println(name)
}
