package browse

import "music-manager/internal/app"

// ProblemView is one finding of the library check.
type ProblemView struct {
	Severity string `json:"severity"` // error or warning
	TrackID  string `json:"trackId"`
	Title    string `json:"title"` // what to call the track
	Message  string `json:"message"`
}

// DoctorView is what a check of the library found.
type DoctorView struct {
	Problems []ProblemView `json:"problems"`
	Errors   int           `json:"errors"`
	Warnings int           `json:"warnings"`
}

// Doctor lays out what a check found, naming each track by its title.
func Doctor(s Snapshot, found []app.Problem) DoctorView {
	v := DoctorView{Problems: []ProblemView{}}
	for _, p := range found {
		sev := "warning"
		if p.Severity == app.Error {
			sev = "error"
			v.Errors++
		} else {
			v.Warnings++
		}
		title := p.TrackID
		if t, ok := s.Track(p.TrackID); ok {
			_, title, _ = app.Display(t)
		}
		v.Problems = append(v.Problems, ProblemView{Severity: sev, TrackID: p.TrackID, Title: title, Message: p.Message})
	}
	return v
}
