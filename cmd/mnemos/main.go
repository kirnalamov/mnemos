package main

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/joho/godotenv"
	"github.com/kirnalamov/mnemos/internal/engine"
	httpserver "github.com/kirnalamov/mnemos/internal/server"
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

	case "compact":
		if err := e.Compact(); err != nil {
			return fmt.Errorf("compact: %w", err)
		}
		fmt.Println("OK")

	case "serve":

		addr := os.Getenv("MNEMOS_ADDR")
		if addr == "" {
			addr = "127.0.0.1:8080"
		}

		srv := httpserver.NewHTTPServer(addr, httpserver.NewHandler(e))

		listener, err := net.Listen("tcp", addr)
		if err != nil {
			return fmt.Errorf("listen on %s: %w", addr, err)
		}

		ctx, stop := signal.NotifyContext(
			context.Background(),
			os.Interrupt,
			syscall.SIGTERM,
		)
		defer stop()

		serverErr := make(chan error, 1)
		go func() {
			serverErr <- srv.Serve(listener)
		}()

		fmt.Println("Mnemos HTTP server listening on", addr)

		select {
		case err := <-serverErr:
			if !errors.Is(err, http.ErrServerClosed) {
				return fmt.Errorf("serve HTTP: %w", err)
			}
			return nil

		case <-ctx.Done():
			shutdownCtx, cancel := context.WithTimeout(
				context.Background(),
				10*time.Second,
			)
			defer cancel()

			if err := srv.Shutdown(shutdownCtx); err != nil {
				return fmt.Errorf("shutdown HTTP server: %w", err)
			}

			err := <-serverErr
			if !errors.Is(err, http.ErrServerClosed) {
				return fmt.Errorf("serve HTTP: %w", err)
			}

			fmt.Println("Mnemos HTTP server stopped")
			return nil
		}
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
	case "compact":
		if len(args) != 1 {
			return errors.New("usage: mnemos compact")
		}
	case "serve":
		if len(args) != 1 {
			return errors.New("usage: mnemos serve")
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
  mnemos compact             Reclaim historical records
  mnemos serve               Start HTTP server
  mnemos help                Show this help`)

}
