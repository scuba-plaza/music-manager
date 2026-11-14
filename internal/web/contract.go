package web

import "music-manager/internal/browse"

func Contract() []any {
	return []any{
		browse.BulkPreview{},
		browse.CopyChoice{},
		browse.DeviceRow{},
		browse.DeviceView{},
		browse.DoctorView{},
		browse.Fetched{},
		browse.IdentifyProgress{},
		browse.PeekView{},
		browse.PlanView{},
		browse.SearchView{},
		browse.TrackView{},
		ChangeResponse{},
		ErrorView{},
		HintView{},
		InfoView{},
		JobView{},
		LibraryResponse{},
		PathView{},
		ProfileView{},
		ServerEvents{},
		SyncData{},
		SyncTargetsView{},
		UnsyncedView{},
	}
}
