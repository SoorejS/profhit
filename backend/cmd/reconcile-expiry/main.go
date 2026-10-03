package main

import (
	"encoding/json"
	"flag"
	"log"
	"os"
	"profhit-backend/config"
	"profhit-backend/services"
)

func main() {
	apply := flag.Bool("apply", false, "Apply reviewed shortening of batch expiries; historical ledger is preserved")
	actor := flag.String("actor", "", "Operator identity required for apply")
	flag.Parse()
	config.ValidateEnv()
	config.ConnectDB()
	if *apply {
		if err := services.ApplyPDFExpiryMigration(*actor); err != nil {
			log.Fatal(err)
		}
		return
	}
	rows, err := services.PreviewPDFExpiryMigration()
	if err != nil {
		log.Fatal(err)
	}
	if err := json.NewEncoder(os.Stdout).Encode(rows); err != nil {
		log.Fatal(err)
	}
}
