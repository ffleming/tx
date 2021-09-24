package main

import (
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
	tickMs        = 500
	statusSeconds = 5
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

type RadioState struct {
	On          bool           `json:"on"`
	TxFrequency string         `json:"frequency" binding:"required"`
	Directory   RadioDirectory `json:"directory" binding:"required"`
	Dial        RadioDial      `json:"dial" binding:"required"`
}

type Radio struct {
	State         *RadioState
	cmd           *exec.Cmd
	mutex         sync.Mutex
	cmdTerminated chan bool
}

func New(fn string) *Radio {
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
		cmdTerminated: make(chan bool),
	}
	go func(*Radio) {
		tick := 0
		for {
			r.sync()
			time.Sleep(tickMs * time.Millisecond)
			if tick%(statusSeconds*1000/tickMs) == 0 {
				r.logStatus()
				tick = 0
			}
			tick++
		}
	}(&r)
	return &r
}

func (r *Radio) Update(state *RadioState) {
	r.mutex.Lock()
	log.Debug("Update()")
	mustHup := r.State.On && (r.State.Dial.Selected != state.Dial.Selected || r.State.TxFrequency != state.TxFrequency)
	r.State = state
	if mustHup {
		log.Debug("Must HUP")
		r.turnOff()
		// Block until command termination writes to the channel
		<-r.cmdTerminated
		r.turnOn()
	}
	r.mutex.Unlock()
}

func (r *Radio) Halt() {
	r.turnOff()
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

func (r *Radio) sync() {
	r.mutex.Lock()
	if r.State.On && !r.broadcasting() {
		log.Debug("sync: turning on")
		r.turnOn()
	} else if !r.State.On && r.broadcasting() {
		log.Debug("sync: turning off")
		r.turnOff()
	}
	r.mutex.Unlock()
}

func (r *Radio) turnOn() {
	if r.broadcasting() {
		log.Error("turnOn() called on a radio that is broadcasting")
	}
	log.Infof("Beginning broadcast on %s FM", r.State.TxFrequency)

	r.State.On = true
	r.cmd = r.playCommand()

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
			if err.Error() != "signal: terminated" {
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
	log.Info("Killed process group %d", p)
	r.cmd = nil
}

func (r *Radio) playCommand() *exec.Cmd {
	sel := r.State.Dial.Selected
	hsh := make(map[string]Station)
	for _, s := range r.State.Directory.Stations {
		hsh[s.Callsign] = s
	}

	station, ok := hsh[sel]
	if !ok {
		log.Errorf("Couldn't find key %q in hash", sel)
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
	cmd := exec.Command("bash", "-c", pipeline)
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
