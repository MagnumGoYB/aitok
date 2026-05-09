package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/MagnumGoYB/aitok/internal/buildinfo"
	"github.com/MagnumGoYB/aitok/internal/cli"
	"github.com/MagnumGoYB/aitok/internal/updatecheck"
)

func main() {
	executable, _ := os.Executable()
	cmd := cli.New(cli.App{
		Out: os.Stdout,
		Err: os.Stderr,
		VersionCheck: func(ctx context.Context, opts cli.VersionCheckOptions) error {
			return updatecheck.MaybeRun(ctx, updatecheck.Options{
				Home:       opts.Home,
				Current:    buildinfo.Version,
				Executable: executable,
				In:         opts.In,
				Err:        opts.Err,
				Now:        func() time.Time { return opts.Now },
			})
		},
		Update: func(ctx context.Context, opts cli.UpdateOptions) error {
			return updatecheck.RunUpdate(ctx, updatecheck.Options{
				Home:       opts.Home,
				Current:    buildinfo.Version,
				Executable: executable,
				In:         opts.In,
				Err:        opts.Err,
				Now:        func() time.Time { return opts.Now },
			})
		},
	})
	if err := cmd.Execute(); err != nil {
		if cli.IsBudgetExceeded(err) {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
