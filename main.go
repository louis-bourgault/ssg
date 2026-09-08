package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/louis-bourgault/ssg/builder"
	"github.com/louis-bourgault/ssg/dev"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
}

func run() error {
	const sourceDir = "routes"
	if len(os.Args) < 2 {
		if err := BuildFromDirectory(sourceDir); err != nil {
			return err
		}
		fmt.Println("Build Completed")
		return nil
	}

	switch subcommand := os.Args[1]; subcommand {
	case "dev":
		flags := flag.NewFlagSet("dev", flag.ContinueOnError)
		host := flags.String("host", "127.0.0.1", "host interface to bind")
		port := flags.Int("port", 8080, "port to bind (0 chooses an ephemeral port)")
		outputDir := flags.String("output", ".ssg-dev", "development output directory")
		if err := flags.Parse(os.Args[2:]); err != nil {
			return err
		}
		server, err := dev.NewServer(dev.Config{
			SourceDir: sourceDir,
			OutputDir: *outputDir,
			Host:      *host,
			Port:      *port,
		})
		if err != nil {
			return err
		}
		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer stop()
		fmt.Printf("Running Development Server on http://%s:%d\n", *host, *port)
		return server.Run(ctx)
	case "serve":
		if err := BuildFromDirectory(sourceDir); err != nil {
			return err
		}
		fmt.Println("Build Completed")
		fmt.Println("Running Static File Server on port 8080")
		fileServer := http.FileServerFS(os.DirFS("build"))
		server := &http.Server{Addr: ":8080", Handler: fileServer}
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("serve build directory: %w", err)
		}
		return nil
	default:
		return fmt.Errorf("unknown command %q; run without a command to build, or use 'dev' or 'serve'", subcommand)
	}
}

// BuildFromDirectory is kept as a small compatibility wrapper for callers of
// the original package-main build helper.
func BuildFromDirectory(sourceDir string) error {
	return builder.Build(context.Background(), builder.Options{SourceDir: sourceDir, OutputDir: "build"})
}
