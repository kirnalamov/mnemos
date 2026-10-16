package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/joho/godotenv"
	"github.com/kirnalamov/mnemos/internal/storage"
)

func main() {
	if err := godotenv.Load(); err != nil {
		fmt.Fprintln(os.Stderr, "Warning: .env file not loaded:", err)
	}

	dataDir := os.Getenv("MNEMOS_DATA_DIR")
	if dataDir == "" {
		fmt.Fprintln(os.Stderr, "MNEMOS_DATA_DIR is not set")
		os.Exit(1)
	}

	if err := os.MkdirAll(dataDir, 0750); err != nil {
		fmt.Fprintf(os.Stderr, "create data directory: %v\n", err)
		os.Exit(1)
	}

	dataPath := filepath.Join(dataDir, "data.mnemos")

	store, err := storage.Open(dataPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "open storage: %v\n", err)
		os.Exit(1)
	}
	defer store.Close()

	fmt.Printf("Mnemos storage opened: %s\n", dataPath)
}
