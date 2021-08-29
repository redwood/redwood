package blob

import (
	"redwood.dev/state"
)

func (s *badgerStore) HelperGetDB() *state.DBTree {
	return s.db
}
