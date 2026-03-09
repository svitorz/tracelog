package main

import (
	"log"

	"svitorz/tracelog/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		log.Fatal(err)
	}
}
