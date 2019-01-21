package main

import (
	"log"

	"github.com/Reidmcc/rockfish/cmd"
)

func main() {
	e := cmd.RootCmd.Execute()
	if e != nil {
		log.Fatal(e)
	}
}
