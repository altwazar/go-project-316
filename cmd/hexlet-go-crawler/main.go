package main

import (
	"code/crawler"
	"context"
	"fmt"
	"github.com/urfave/cli/v3"
	"log"
	"net/http"
	"os"
	"time"
)

func main() {
	cmd := &cli.Command{
		Name:      "hexlet-go-crawler",
		Usage:     "[global options] command [command options] <url>",
		UsageText: "hexlet-path-size [global options]",
		Flags: []cli.Flag{
			&cli.IntFlag{
				Name:  "depth",
				Value: 10,
				Usage: "crawl depth (default: 10)",
			},
			&cli.IntFlag{
				Name:  "retries",
				Value: 1,
				Usage: "number of retries for failed requests (default: 1)",
			},
			&cli.DurationFlag{
				Name:  "delay",
				Value: 0 * time.Second,
				Usage: "delay between requests (example: 200ms, 1s) (default: 0s)",
			},
			&cli.DurationFlag{
				Name:  "timeout",
				Value: 15 * time.Second,
				Usage: "per-request timeout (default: 15s)",
			},
			&cli.IntFlag{
				Name:  "rps",
				Value: 0,
				Usage: "limit requests per second (overrides delay) (default: 0 - no limit)",
			},
			&cli.StringFlag{
				Name:  "user-agent",
				Value: "",
				Usage: "custom user agent",
			},
			&cli.IntFlag{
				Name:  "workers",
				Value: 4,
				Usage: "number of concurrent workers (default: 4)",
			},
		},
		Action: func(ctx context.Context, cmd *cli.Command) error {
			if cmd.NArg() != 1 {
				err := cli.ShowAppHelp(cmd)
				if err != nil {
					log.Fatal(err)
				}
				return cli.Exit("Error: requires one argument - url", 1)
			}

			delay := cmd.Duration("delay")
			rps := cmd.Int("rps")

			if rps > 0 {
				delay = 0
			}

			opts := crawler.Options{
				URL:         cmd.Args().Get(0),
				Depth:       cmd.Int("depth"),
				Delay:       delay,
				Timeout:     cmd.Duration("timeout"),
				Retries:     cmd.Int("retries"),
				UserAgent:   cmd.String("user-agent"),
				Concurrency: cmd.Int("workers"),
				RPS:         rps,
				HTTPClient: &http.Client{
					Timeout: cmd.Duration("timeout"),
				},
			}

			out, err := crawler.Analyze(ctx, opts)
			if err != nil {
				log.Fatal(err)
			}
			fmt.Println(string(out))
			return nil
		},
	}

	if err := cmd.Run(context.Background(), os.Args); err != nil {
		log.Fatal(err)
	}
}
