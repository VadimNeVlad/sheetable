package main

import (
	"log"

	"github.com/SheetAble/SheetAble/backend/api"
	"github.com/SheetAble/SheetAble/backend/api/utils"
)

func main() {
	utils.Version = "v0.8.1"
	utils.PrintAsciiVersion()
	if err := api.Run(); err != nil {
		log.Fatalf("server stopped with an error: %v", err)
	}
}
