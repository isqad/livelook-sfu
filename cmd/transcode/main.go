package main

import (
	"fmt"
	"os"

	"github.com/isqad/livelook-sfu/internal/transcode"
	"github.com/urfave/cli/v2"
)

func main() {
	app := &cli.App{
		Name:        "livelook-transcode",
		Usage:       "Transcode service",
		Description: "",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:  "natsAddr",
				Value: "nats://127.0.0.1:10222",
				Usage: "Address to connect to NATS server",
			},
		},
		Action: start,
	}

	if err := app.Run(os.Args); err != nil {
		fmt.Printf("%v\n", err)
	}
}

func start(c *cli.Context) error {
	daemon, err := transcode.New(c.String("natsAddr"))
	if err != nil {
		return err
	}

	if err := daemon.Run(); err != nil {
		return err
	}

	return nil
}
