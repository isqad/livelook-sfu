package main

import (
	"fmt"
	"os"

	"github.com/isqad/livelook-sfu/internal/transcode"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/viper"
	"github.com/urfave/cli/v2"
)

const (
	developmentEnv = "development"
)

func main() {
	appEnv := os.Getenv("APP_ENV")
	if appEnv == "" {
		appEnv = developmentEnv
	}

	zerolog.SetGlobalLevel(zerolog.InfoLevel)
	if appEnv == developmentEnv {
		log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	}

	viper.SetConfigName("config." + appEnv)
	viper.SetConfigType("yaml")
	viper.AddConfigPath("./configs")
	err := viper.ReadInConfig()
	if err != nil {
		log.Fatal().Err(err).Msg("can not read config file")
	}

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
