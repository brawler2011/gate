// Command judgeimage prints the judge image name (minimal by default, or full) for the current judge/ sources, or
// with the argument typeset the image built from typeset/.
package main

import (
	"fmt"
	"os"

	"github.com/brawler2011/gate/backend/tests/judgeimage"
)

func main() {
	ref := judgeimage.Ref
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "minimal":
		case "full":
			ref = judgeimage.FullRef
		case "typeset":
			ref = judgeimage.TypesetRef
		default:
			fmt.Fprintln(os.Stderr, "unknown image", os.Args[1])
			os.Exit(2)
		}
	}
	name, err := ref()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Println(name)
}
