package sub

import (
	"time"

	"tungo/internal/appdir"
)

type Item struct {
	Name     string    `json:"name"`
	URL      string    `json:"url"`
	Interval int       `json:"interval,omitempty"`
	Updated  time.Time `json:"updated"`
}

type State struct {
	Current string `json:"current,omitempty"`
	Items   []Item `json:"items"`
}

func Load() State {
	var state State
	_ = appdir.ReadJSON("subscription.json", &state)
	return state
}

func Save(state State) error { return appdir.WriteJSON("subscription.json", state) }

func (s State) Item(name string) *Item {
	for i := range s.Items {
		if s.Items[i].Name == name {
			return &s.Items[i]
		}
	}
	return nil
}

func (s State) Running() *Item {
	if item := s.Item(s.Current); item != nil {
		return item
	}
	if len(s.Items) > 0 {
		return &s.Items[0]
	}
	return nil
}

func (s *State) Put(item Item) {
	if kept := s.Item(item.Name); kept != nil {
		*kept = item
		return
	}
	s.Items = append(s.Items, item)
}

func (s *State) Remove(name string) {
	kept := make([]Item, 0, len(s.Items))
	for _, item := range s.Items {
		if item.Name != name {
			kept = append(kept, item)
		}
	}
	s.Items = kept
	if s.Item(s.Current) == nil {
		s.Current = ""
		if len(s.Items) > 0 {
			s.Current = s.Items[0].Name
		}
	}
}
