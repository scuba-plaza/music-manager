package tui

import (
	"os/exec"
	"syscall"
)

// Player plays an audio file. It is how a trim is checked by ear.
type Player interface {
	// Play starts playing path and returns at once. stop ends the sound;
	// done receives once when it has ended, however it ended.
	Play(path string) (stop func(), done <-chan error, err error)
}

// execPlayer plays files by running another program.
type execPlayer struct {
	bin  string
	args func(path string) []string
}

// DetectPlayer returns a player that uses ffplay or mpv, whichever is
// installed (ffplay first), or nil if neither is.
func DetectPlayer() Player {
	if bin, err := exec.LookPath("ffplay"); err == nil {
		return &execPlayer{bin, func(path string) []string {
			return []string{"-nodisp", "-autoexit", "-loglevel", "quiet", "-hide_banner", path}
		}}
	}
	if bin, err := exec.LookPath("mpv"); err == nil {
		return &execPlayer{bin, func(path string) []string {
			return []string{"--no-video", "--really-quiet", "--no-terminal", path}
		}}
	}
	return nil
}

func (p *execPlayer) Play(path string) (func(), <-chan error, error) {
	cmd := exec.Command(p.bin, p.args(path)...)
	// A group of its own, so that stopping takes the player's helpers with it.
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := cmd.Start(); err != nil {
		return nil, nil, err
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	stop := func() { _ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL) }
	return stop, done, nil
}
