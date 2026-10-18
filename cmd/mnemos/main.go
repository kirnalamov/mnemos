package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/joho/godotenv"
	"github.com/kirnalamov/mnemos/internal/engine"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
}

func run(args []string) (retErr error) {
	if len(args) == 0 {
		printUsage()
		return nil
	}

	if args[0] == "help" || args[0] == "--help" || args[0] == "-h" {
		printUsage()
		return nil
	}

	if err := validateArgs(args); err != nil {
		return err
	}

	if err := godotenv.Load(); err != nil {
		fmt.Fprintln(os.Stderr, "Warning: .env file not loaded:", err)
	}

	dataDir := os.Getenv("MNEMOS_DATA_DIR")
	if dataDir == "" {
		return errors.New("MNEMOS_DATA_DIR is not set")
	}

	if err := os.MkdirAll(dataDir, 0750); err != nil {
		return fmt.Errorf("create data directory: %w", err)
	}

	dataPath := filepath.Join(dataDir, "data.mnemos")

	e, err := engine.Open(dataPath)
	if err != nil {
		return fmt.Errorf("open storage: %w", err)
	}

	defer func() {
		if err := e.Close(); err != nil {
			closeErr := fmt.Errorf("close storage: %w", err)
			retErr = errors.Join(retErr, closeErr)
		}
	}()

	switch args[0] {
	case "put":
		if err := e.Put(args[1], []byte(args[2])); err != nil {
			return fmt.Errorf("put: %w", err)
		}
		fmt.Println("OK")

	case "get":
		data, err := e.Get(args[1])
		if err != nil {
			return fmt.Errorf("get: %w", err)
		}
		fmt.Println(string(data))

	case "delete":
		if err := e.Delete(args[1]); err != nil {
			return fmt.Errorf("delete: %w", err)
		}
		fmt.Println("OK")
	}

	return nil
}

func validateArgs(args []string) error {
	switch args[0] {
	case "put":
		if len(args) != 3 {
			return errors.New("usage: mnemos put <key> <value>")
		}
	case "get":
		if len(args) != 2 {
			return errors.New("usage: mnemos get <key>")
		}
	case "delete":
		if len(args) != 2 {
			return errors.New("usage: mnemos delete <key>")
		}
	default:
		return fmt.Errorf("unknown command %q; run 'mnemos help' for usage", args[0])
	}

	return nil
}

func printUsage() {
	fmt.Println(`Mnemos - local key-value storage

Usage:
  mnemos put <key> <value>   Store a value
  mnemos get <key>           Read a value
  mnemos delete <key>        Delete a key
  mnemos help                Show this help`)
}
