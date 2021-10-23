package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os/exec"
	"sync"
	"syscall"
	"time"

	log "github.com/sirupsen/logrus"
	"gopkg.in/alessio/shellescape.v1"
)

const (
	tickDur = 50 * time.Millisecond
	syncDur = 500 * time.Millisecond

	statusDur = 5 * time.Second
)

type Station struct {
	Callsign  string `json:"callsign" binding:"required"`
	URL       string `json:"url" binding:"required"`
	Frequency string `json:"frequency" binding:"required"`
	Info      string `json:"info" binding:"required"`
}

type RadioDial struct {
	Selected string   `json:"selected" binding:"required"`
	Stations []string `json:"stations" binding:"required"`
}

type RadioDirectory struct {
	Stations []Station `json:"stations" binding:"required"`
}

func (rd *RadioDirectory) Lookup(callsign string) (Station, error) {
	for _, st := range rd.Stations {
		if st.Callsign == callsign {
			return st, nil
		}
	}
	return Station{}, fmt.Errorf("Station with callsign %q not found", callsign)
}

type RadioState struct {
	On          bool           `json:"on"`
	TxFrequency string         `json:"frequency" binding:"required"`
	Directory   RadioDirectory `json:"directory" binding:"required"`
	Dial        RadioDial      `json:"dial" binding:"required"`
}

type Radio struct {
	State         *RadioState
	display       RadioDisplay
	filename      string
	cmd           *exec.Cmd
	mutex         sync.Mutex
	cmdTerminated chan bool
}

func NewRadio(ctx context.Context, fn string, rd RadioDisplay) *Radio {
	var rs RadioState
	jsonConf, err := ioutil.ReadFile(fn)
	if err != nil {
		log.Fatalf("Couldn't %s", err)
	}

	err = json.Unmarshal(jsonConf, &rs)
	if err != nil {
		log.Fatalf("Couldn't parse JSON in %s: %s", jsonConf, err)
	}

	r := Radio{
		State:         &rs,
		display:       rd,
		filename:      fn,
		cmdTerminated: make(chan bool),
	}
	go func(ctx context.Context, r *Radio) {
		var lastSync, lastStatus time.Time
		for {
			select {
			case <-ctx.Done():
				log.Info("Radio shutting down")
				if r.broadcasting() {
					r.turnOff()
				}
				return
			default:
				now := time.Now()
				if lastSync.Add(syncDur).Before(now) {
					r.sync(ctx)
					lastSync = now
				}
				if lastStatus.Add(statusDur).Before(now) {
					r.logStatus()
					lastStatus = now
				}
				r.updateScreen()
				time.Sleep(tickDur)
			}
		}
	}(ctx, &r)
	return &r
}

func (r *Radio) Update(ctx context.Context, state *RadioState) {
	r.mutex.Lock()
	log.Debug("Update()")
	mustHup := r.State.On && (r.State.Dial.Selected != state.Dial.Selected || r.State.TxFrequency != state.TxFrequency)
	r.State = state
	r.saveState()
	if mustHup {
		log.Debug("Must HUP")
		r.turnOff()
		// Block until command termination writes to the channel
		<-r.cmdTerminated
		r.turnOn(ctx)
	}
	r.mutex.Unlock()
}

func (r *Radio) broadcasting() bool {
	if r.cmd == nil {
		return false
	}
	if r.cmd.Process == nil {
		return false
	}
	if r.cmd.ProcessState != nil && r.cmd.ProcessState.Exited() {
		return false
	}
	if r.cmd.Process.Pid != 0 {
		return true
	}
	return false
}

func (r *Radio) saveState() {
	b, err := json.MarshalIndent(r.State, "", "  ")
	if err != nil {
		log.Error(err)
		return
	}

	if err = ioutil.WriteFile(r.filename, b, 0644); err != nil {
		log.Error(err)
	}
}

func (r *Radio) sync(ctx context.Context) {
	r.mutex.Lock()
	if r.State.On && !r.broadcasting() {
		log.Debug("sync: turning on")
		r.turnOn(ctx)
	} else if !r.State.On && r.broadcasting() {
		log.Debug("sync: turning off")
		r.turnOff()
	}
	r.mutex.Unlock()
}

func (r *Radio) turnOn(ctx context.Context) {
	if r.broadcasting() {
		log.Error("turnOn() called on a radio that is broadcasting")
	}
	log.Infof("Beginning broadcast on %s FM", r.State.TxFrequency)

	r.State.On = true
	r.cmd = r.playCommand(ctx)

	// Ensure that cmd.Process is set before we start goroutine
	if err := r.cmd.Start(); err != nil {
		log.Error(err)
	}
	go func() {
		// Empty channel so that an unconsumed value can't lock us
		// This obviates the need for an early return in the case of turnOn()
		// being called on a broadcasting radio
		for len(r.cmdTerminated) > 0 {
			<-r.cmdTerminated
		}

		if err := r.cmd.Wait(); err != nil {
			if err.Error() != "signal: terminated" && err.Error() != "signal: killed" {
				log.Errorf("Error in run: %s", err)
			}
		}
		// Signal that command has terminated
		r.cmdTerminated <- true
	}()
}

func (r *Radio) turnOff() {
	if !r.broadcasting() {
		log.Error("turnOff() called on a radio that isn't broadcasting")
		return
	}
	r.State.On = false

	p := -r.cmd.Process.Pid
	if err := syscall.Kill(p, syscall.SIGTERM); err != nil {
		log.Errorf("Error in kill: %s", err)
	}
	log.Infof("Killed process group %d", p)
	r.cmd = nil
}

func (r *Radio) playCommand(ctx context.Context) *exec.Cmd {
	station, err := r.State.Directory.Lookup(r.State.Dial.Selected)
	if err != nil {
		log.Error(err)
		return nil
	}

	pipeline := fmt.Sprintf(
		"/usr/bin/sox -t mp3 %s -t wav - | /usr/bin/sudo /home/fsf/PiFmRds/src/pi_fm_rds -freq %s -audio -",
		shellescape.Quote(station.URL),
		shellescape.Quote(r.State.TxFrequency),
	)
	if *fNoTx {
		pipeline = "cat /dev/random | /usr/bin/sudo tail -f"
	}
	cmd := exec.CommandContext(ctx, "bash", "-c", pipeline)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	return cmd
}

func (r *Radio) logStatus() {
	log.Debugf("Broadcasting: %t", r.broadcasting())
	log.Debugf("Cmd: %+v", r.cmd)
	if r.cmd != nil {
		log.Debugf("Process: %+v", r.cmd.Process)
		if r.cmd.Process != nil {
			log.Debugf("ProcessState %+v", r.cmd.ProcessState)
		}
	}
}

func (r *Radio) updateScreen() {
	var pages []DisplayPage

	if !r.State.On {
		pages = []DisplayPage{
			{
				Line1:    "Off",
				Duration: 10000 * time.Millisecond,
			},
		}
		r.display.Tick(pages)
		return
	}

	station, err := r.State.Directory.Lookup(r.State.Dial.Selected)
	if err != nil {
		log.Error(err)
		return
	}

	pages = []DisplayPage{
		{
			Line1:    station.Frequency,
			Line2:    station.Callsign,
			Duration: 5000 * time.Millisecond,
		},
		{
			Line1:    "Broadcasting on",
			Line2:    fmt.Sprintf("%s FM", r.State.TxFrequency),
			Duration: 2000 * time.Millisecond,
		},
	}
	r.display.Tick(pages)
}
