package main

import (
	"log"

	"github.com/cwarden/urd/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		log.Fatal(err)
	}
}
