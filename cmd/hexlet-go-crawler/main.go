package main

import (
	"code"
	"context"
	"fmt"
	"github.com/urfave/cli/v3" // imports as package "cli"
	"log"
	"os"
)

func main() {

	cmd := &cli.Command{
		Name:      "hexlet-go-crawler",
		Usage:     "[global options] command [command options] <url>",
		UsageText: "hexlet-path-size [global options]",
		Flags: []cli.Flag{
			&cli.UintFlag{
				Name:  "depth",
				Value: 10,
				Usage: "crawl depth (default: 10)"},
			&cli.UintFlag{
				Name:  "retries",
				Value: 1,
				Usage: "number of retries for failed requests (default: 1)"},
			&cli.StringFlag{
				Name:  "delay",
				Value: "0s",
				Usage: "delay between requests (example: 200ms, 1s) (default: 0s)"},
			&cli.StringFlag{
				Name:  "timeout",
				Value: "15s",
				Usage: "per-request timeout (default: 15s)"},
			&cli.UintFlag{
				Name:  "delay",
				Value: 0,
				Usage: "limit requests per second (overrides delay) (default: 0)"},
			&cli.StringFlag{
				Name:  "user-agent",
				Value: "",
				Usage: "custom user agent"},
			&cli.UintFlag{
				Name:  "workers",
				Value: 4,
				Usage: "number of concurrent workers (default: 4)"},
		},
		Action: func(ctx context.Context, cmd *cli.Command) error {
			// Нужен один аргумент - url
			if cmd.NArg() != 1 {
				err := cli.ShowAppHelp(cmd)

				if err != nil {
					log.Fatal(err)
				}
				return cli.Exit("Error: requires one argument - url", 1)
			}

			opts := code.Options{
				URL:   cmd.Args().Get(0),
				Depth: cmd.Uint("depth"),
			}
			out, err := code.Analyze(ctx, opts)
			if err == nil {

				fmt.Println(string(out))
			}
			if err != nil {
				log.Fatal(err)
			}
			return nil
		},
	}
	if err := cmd.Run(context.Background(), os.Args); err != nil {
		log.Fatal(err)
	}

}
