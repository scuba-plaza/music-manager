package tui

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"music-manager/internal/app"
)

// previewWindow is how much of a cut is played: this much of the kept audio
// beside it.
const previewWindow = 8 * time.Second

// playback is a preview that is playing now.
type playback struct {
	id      int
	edge    app.Edge
	stop    func()
	cleanup func()
}

// playbackStartedMsg says the preview has started, or why it did not.
type playbackStartedMsg struct {
	pb   *playback
	done <-chan error
	err  error
}

// playbackDoneMsg says a preview ended on its own.
type playbackDoneMsg struct{ id int }

// stopPlayback ends the preview that is playing, if any, and removes its file.
func (m *Model) stopPlayback() {
	if m.playing == nil {
		return
	}
	m.playing.stop()
	m.playing.cleanup()
	m.playing = nil
}

// togglePreview plays the audio around the cut being looked at, or stops it.
func (m Model) togglePreview() (tea.Model, tea.Cmd) {
	f := trackFields[m.field]
	if !hearable(f) {
		return m, nil
	}
	if m.playing != nil {
		m.stopPlayback()
		return m, nil
	}
	if m.player == nil {
		m.say("To hear a preview, install ffplay (part of ffmpeg) or mpv", true)
		return m, nil
	}
	r, ok := m.selectedRow()
	if !ok {
		return m, nil
	}
	edge := app.EdgeStart
	if f == fCutEnd || f == fFadeOut {
		edge = app.EdgeEnd
	}
	window := previewWindow
	switch f { // a fade is heard whole, and a little beyond
	case fFadeIn:
		window = max(window, r.Track.FadeIn.End-r.Track.FadeIn.Start+2*time.Second)
	case fFadeOut:
		window = max(window, r.Track.FadeOut.End-r.Track.FadeOut.Start+2*time.Second)
	}
	m.previews++
	id, lib, player, trackID := m.previews, m.lib, m.player, r.Track.ID
	return m, func() tea.Msg {
		path, cleanup, err := lib.PreviewTrim(trackID, edge, window)
		if err != nil {
			return playbackStartedMsg{err: err}
		}
		stop, done, err := player.Play(path)
		if err != nil {
			cleanup()
			return playbackStartedMsg{err: err}
		}
		return playbackStartedMsg{pb: &playback{id: id, edge: edge, stop: stop, cleanup: cleanup}, done: done}
	}
}

func (m Model) playbackStarted(msg playbackStartedMsg) (tea.Model, tea.Cmd) {
	if msg.err != nil {
		m.say(msg.err.Error(), true)
		return m, nil
	}
	m.playing = msg.pb
	done, id := msg.done, msg.pb.id
	return m, func() tea.Msg {
		<-done
		return playbackDoneMsg{id}
	}
}

func (m Model) playbackDone(msg playbackDoneMsg) (tea.Model, tea.Cmd) {
	if m.playing != nil && m.playing.id == msg.id {
		m.playing.cleanup()
		m.playing = nil
	}
	return m, nil
}

// hearable reports whether the field has a preview: the cuts, the volume and
// the fades are things to hear.
func hearable(f fieldKind) bool {
	switch f {
	case fCutStart, fCutEnd, fVolume, fFadeIn, fFadeOut:
		return true
	}
	return false
}
