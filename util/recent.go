package main

import (
	"os"
	"fmt"
)

func main() {
	//Read the recent notifications page  and decrypt the content for grins
	if len(os.Args) != 2 {
		fmt.Printf("Usage: %s host:port\n", os.Args[0])
		return
	}
}
