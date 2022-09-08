package main

import (
	"os"

	"github.com/rs/zerolog/log"

	"github.com/urfave/cli/v2"

	"github.com/isqad/livelook-sfu/internal/core"
	"github.com/isqad/livelook-sfu/internal/ws"
)

func main() {
	app := &cli.App{
		Name:        "livelook-ws",
		Usage:       "Websocket server",
		Description: "",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:     "env",
				Usage:    "environment: either 'development' or 'production'",
				Required: true,
			},
			&cli.StringFlag{
				Name:  "address",
				Usage: "listen IP and port, example: ':80' (default value) for listen on 0.0.0.0:80",
				Value: ":80",
			},
		},
		Action: startWs,
	}

	if err := app.Run(os.Args); err != nil {
		log.Fatal().Err(err).Msg("")
	}
}

func startWs(c *cli.Context) error {
	wsApp := ws.New(ws.WsAppOptions{
		Address: c.String("address"),
		Env:     core.Environment(c.String("env")),
	})

	return wsApp.Start()
}
