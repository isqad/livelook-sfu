package transcode

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"

	"github.com/isqad/livelook-sfu/internal/core"
	"github.com/nats-io/nats.go"
	"github.com/rs/zerolog/log"
	"github.com/spf13/viper"
)

type Daemon struct {
	sync.RWMutex
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
		nc:            nc,
		errors:        make(chan error),
		stop:          make(chan struct{}),
		tanscoderPids: make(map[core.UserSessionID]int),
	}

	return daemon, nil
}

func (d *Daemon) Run() error {
	log.Info().Msg("start transcode daemon")

	var err error

	// подписка на запуск транскодера, сообщение получает только один из запущенных сервисов
	d.startTranscoderSub, err = d.nc.QueueSubscribe(TranscoderStartSubj, TranscoderQueueHLS, func(msg *nats.Msg) {
		go d.handleStartTranscoderMessage(msg, d.errors)
	})
	if err != nil {
		d.Stop()
		return err
	}

	// подписка на остановку транскодера, сообщение получают все сервисы, т.к. не известно, какой сервис запустил данный
	// транскодер
	d.stopTranscoderSub, err = d.nc.Subscribe(TranscoderStopSubj, func(msg *nats.Msg) {
		go d.handleStopTranscoderMessage(msg, d.errors)
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

	if err := d.nc.Drain(); err != nil {
		log.Error().Err(err).Msg("")
	}
}

func (d *Daemon) handleStartTranscoderMessage(msg *nats.Msg, errChan chan<- error) {
	log.Debug().Str("data", string(msg.Data[:])).Msg("received message, try to start transcoder")

	payload := &Message{}

	r := bytes.NewReader(msg.Data)
	err := json.NewDecoder(r).Decode(payload)
	if err != nil {
		errChan <- fmt.Errorf("transcode error: %v, payload: %s", err, string(msg.Data[:]))
		return
	}

	if err := d.startTranscoder(payload); err != nil {
		errChan <- err
	}
}

func (d *Daemon) handleStopTranscoderMessage(msg *nats.Msg, errChan chan<- error) {
	payload := &Message{}

	r := bytes.NewReader(msg.Data)
	err := json.NewDecoder(r).Decode(payload)
	if err != nil {
		errChan <- fmt.Errorf("transcode error: %v, payload: %s", err, string(msg.Data[:]))
		return
	}

	if err := d.stopTranscoder(payload.UserID); err != nil {
		errChan <- err
	}
}

func (d *Daemon) startTranscoder(payload *Message) error {
	rootDir := viper.GetString("app.streams_root_dir")
	userDir := rootDir + "/" + string(payload.UserID)
	streamDir := rootDir + "/" + string(payload.UserID) + "/stream"

	if err := os.MkdirAll(userDir, 0755); err != nil {
		return err
	}
	if err := os.MkdirAll(streamDir, 0755); err != nil {
		return err
	}

	sdpFilePath := userDir + "/transcoder.sdp"

	f, err := os.Create(sdpFilePath)
	if err != nil {
		return err
	}

	// defer f.Close() нельзя, так как файл будет открытым пока идет трансляция
	if _, err := f.Write(payload.SDP); err != nil {
		f.Close()
		return err
	}
	f.Close()

	m3u8FilePath := streamDir + "/stream.m3u8"

	ffmpegCmd := exec.Command(
		"ffmpeg",
		"-protocol_whitelist", "file,udp,rtp",
		"-i", sdpFilePath,
		"-c:v", "libx264",
		"-preset", "veryfast",
		"-crf", "18",
		"-b:v", "3000k",
		"-maxrate", "3000k",
		"-bufsize", "6000k",
		"-pix_fmt", "yuv420p",
		"-g", "30",
		"-flags", "low_delay",
		"-hls_time", "2",
		"-hls_flags", "delete_segments",
		"-hls_list_size", "5",
		m3u8FilePath,
	)

	ffmpegCmdIn, err := ffmpegCmd.StdinPipe()
	if err != nil {
		return err
	}
	ffmpegCmdIn.Close()

	ffmpegCmdOut, err := ffmpegCmd.StdoutPipe()
	if err != nil {
		return err
	}

	go func() {
		if _, err := io.Copy(os.Stdout, ffmpegCmdOut); err != nil {
			log.Error().Err(err).Msg("close ffmpeg cmd out")
		}
	}()

	err = ffmpegCmd.Start()
	if err != nil {
		return err
	}

	pid := ffmpegCmd.Process.Pid

	log.Info().Int("pid", pid).Msg("ffmpeg started")

	d.Lock()
	d.tanscoderPids[payload.UserID] = pid
	d.Unlock()

	if err := ffmpegCmd.Wait(); err != nil {
		log.Error().Err(err).Msg("ffmpeg error")
	}

	return nil
}

func (d *Daemon) stopTranscoder(userID core.UserSessionID) error {
	var pid int

	d.RLock()
	pid, ok := d.tanscoderPids[userID]
	if !ok {
		d.RUnlock()
		return nil
	}
	d.RUnlock()

	process, err := os.FindProcess(pid)
	if err != nil {
		return err
	}
	if err := process.Kill(); err != nil {
		log.Error().Err(err).Msg("ffmpeg error")
	}

	d.Lock()
	delete(d.tanscoderPids, userID)
	defer d.Unlock()

	log.Info().Int("pid", pid).Msg("ffmpeg stopped")

	rootDir := viper.GetString("app.streams_root_dir")
	userDir := rootDir + "/" + string(userID)
	if err := os.RemoveAll(userDir); err != nil {
		return err
	}

	return nil
}
