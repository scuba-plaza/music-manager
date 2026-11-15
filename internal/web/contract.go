package web

import (
	"music-manager/internal/browse"
	"music-manager/internal/manifest"
)

func Contract() []any {
	return []any{
		browse.BulkPreview{},
		browse.CopyChoice{},
		browse.DeviceRow{},
		browse.DeviceView{},
		browse.DoctorView{},
		browse.Fetched{},
		browse.PeekView{},
		browse.PlanView{},
		browse.TrackView{},
		manifest.Profile{},
		AddCheck{},
		ChangeResponse{},
		ErrorView{},
		HintView{},
		InfoView{},
		JobView{},
		LibraryResponse{},
		ProfileCheck{},
		PathView{},
		ServerEvents{},
		SyncData{},
		SyncTargetsView{},
	}
}
