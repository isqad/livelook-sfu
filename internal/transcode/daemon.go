package transcode

import (
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/nats-io/nats.go"
	"github.com/rs/zerolog/log"
)

type Daemon struct {
	nc  *nats.Conn
	sub *nats.Subscription

	errors chan error
	stop   chan struct{}
}

func New(natsAddr string) (*Daemon, error) {
	nc, err := nats.Connect(natsAddr, nats.NoEcho())
	if err != nil {
		return nil, err
	}

	daemon := &Daemon{
		nc:     nc,
		errors: make(chan error),
		stop:   make(chan struct{}),
	}

	return daemon, nil
}

func (d *Daemon) Run() error {
	log.Info().Msg("start transcode daemon")

	var err error
	d.sub, err = d.nc.QueueSubscribe(TranscodeSubcriptionSubject, TranscodeSubcriptionQueueHLS, func(msg *nats.Msg) {
		if err := d.startTranscoder(msg); err != nil {
			d.errors <- err
		}
	})
	if err != nil {
		return err
	}

	for {
		select {
		case err := <-d.errors:
			log.Error().Err(err).Msg("")
		case <-d.stop:
			return d.Stop()
		}
	}
}

func (d *Daemon) Stop() error {
	log.Info().Msg("stop transcode daemon")

	if err := d.sub.Unsubscribe(); err != nil {

	}

	return d.nc.Drain()
}

func (d *Daemon) startTranscoder(msg *nats.Msg) error {
	log.Debug().Str("data", string(msg.Data[:])).Msg("received message, try to start transcoder")
	defer func() { log.Debug().Msg("stop transcoder") }()

	payload := &Messsage{}

	r := bytes.NewReader(msg.Data)
	err := json.NewDecoder(r).Decode(payload)
	if err != nil {
		return fmt.Errorf("transcode error: %v, payload: %s", err, string(msg.Data[:]))
	}

	return nil
}
