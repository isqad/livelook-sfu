package transcode

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"sync"

	"github.com/isqad/livelook-sfu/internal/core"
	"github.com/nats-io/nats.go"
	"github.com/rs/zerolog/log"
	"github.com/spf13/viper"
)

type Daemon struct {
	sync.Mutex
	nc                 *nats.Conn
	startTranscoderSub *nats.Subscription
	stopTranscoderSub  *nats.Subscription

	tanscoderPids map[core.UserSessionID]int

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
		tanscoderPids: make(map[core.UserSessionID]int),
	}

	return daemon, nil
}

func (d *Daemon) Run() error {
	log.Info().Msg("start transcode daemon")

	var err error

	// подписка на запуск транскодера, сообщение получает только один из запущенных сервисов
	d.startTranscoderSub, err = d.nc.QueueSubscribe(TranscoderStartSubj, TranscoderQueueHLS, func(msg *nats.Msg) {
		go handleStartTranscoderMessage(msg, d.errors)
	})
	if err != nil {
		d.Stop()
		return err
	}

	// подписка на остановку транскодера, сообщение получают все сервисы, т.к. не известно, какой сервис запустил данный
	// транскодер
	d.stopTranscoderSub, err = d.nc.Subscribe(TranscoderStopSubj, func(msg *nats.Msg) {
		go handleStopTranscoderMessage(msg, d.errors)
	})
	if err != nil {
		d.Stop()
		return err
	}

	for {
		select {
		case err := <-d.errors:
			log.Error().Err(err).Msg("")
		case <-d.stop:
			d.Stop()
			return nil
		}
	}
}

func (d *Daemon) Stop() {
	log.Info().Msg("stop transcode daemon")

	if err := d.startTranscoderSub.Unsubscribe(); err != nil {
		log.Error().Err(err).Msg("")
	}

	if err := d.stopTranscoderSub.Unsubscribe(); err != nil {
		log.Error().Err(err).Msg("")
	}

	if err := d.nc.Drain(); err !=nil {
		log.Error().Err(err).Msg("")
	}
}

func handleStartTranscoderMessage(msg *nats.Msg, errChan chan<- error) {
	log.Debug().Str("data", string(msg.Data[:])).Msg("received message, try to start transcoder")

	payload := &Message{}

	r := bytes.NewReader(msg.Data)
	err := json.NewDecoder(r).Decode(payload)
	if err != nil {
		errChan <- fmt.Errorf("transcode error: %v, payload: %s", err, string(msg.Data[:]))
		return
	}


	if err := startTranscoder(payload); err != nil {
		errChan <- err
	}
}

func handleStopTranscoderMessage(msg *nats.Msg, errChan chan<- error) {
}

func startTranscoder(payload *Message) error {
	rootDir := viper.GetString("app.streams_root_dir")
	if err := os.MkdirAll(rootDir+"/"+string(payload.UserID), 0755); err != nil {
		return err
	}

	f, err := os.Create(rootDir + "/" + string(payload.UserID) + "/transcoder.sdp")
	if err != nil {
		return err
	}

	// defer f.Close() нельзя, так как файл будет открытым пока идет трансляция
	if _, err := f.Write(payload.SDP); err != nil {
		f.Close()
		return err
	}
	f.Close()

	return nil
}

func stopTranscoder(userID core.UserSessionID) error {
	return nil
}
