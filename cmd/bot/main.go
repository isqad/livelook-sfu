package main

import (
	"fmt"
	"os"

	"github.com/isqad/livelook-sfu/internal/bot"
	"github.com/urfave/cli/v2"
)

func main() {
	app := &cli.App{
		Name:        "livelook-bot",
		Usage:       "WebRTC bot for streaming media to livelook",
		Description: "",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:  "host",
				Value: "localhost:3001",
				Usage: "main host of server",
			},
			&cli.StringFlag{
				Name:     "username",
				Usage:    "username for authenticate",
				Required: true,
			},
			&cli.StringFlag{
				Name:     "password",
				Usage:    "password for authenticate",
				Required: true,
			},
		},
		Action: startBot,
	}

	if err := app.Run(os.Args); err != nil {
		fmt.Printf("%v\n", err)
	}
}

func startBot(c *cli.Context) error {
	bot, err := bot.New(c.String("host"), c.String("username"), c.String("password"))
	if err != nil {
		return nil
	}

	if err := bot.Start(); err != nil {
		return err
	}

	return nil
}
