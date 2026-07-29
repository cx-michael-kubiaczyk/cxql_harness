package harness

type HistoryFilter struct {
	System    bool
	Changelog bool
	Notes     bool
	Messages  bool
}

var FilterAll = HistoryFilter{
	System:    true,
	Changelog: true,
	Notes:     true,
	Messages:  true,
}
var FilterNone = HistoryFilter{}

func (f HistoryFilter) Except(filter HistoryFilter) HistoryFilter {
	return HistoryFilter{
		System:    f.System && !filter.System,
		Changelog: f.Changelog && !filter.Changelog,
		Notes:     f.Notes && !filter.Notes,
		Messages:  f.Messages && !filter.Messages,
	}
}

func (f HistoryFilter) And(filter HistoryFilter) HistoryFilter {
	return HistoryFilter{
		System:    f.System || filter.System,
		Changelog: f.Changelog || filter.Changelog,
		Notes:     f.Notes || filter.Notes,
		Messages:  f.Messages || filter.Messages,
	}
}
