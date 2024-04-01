package main

import (
	"log"

	"github.com/azhovan/durable-resume/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		log.Fatalln(err)
	}
}
