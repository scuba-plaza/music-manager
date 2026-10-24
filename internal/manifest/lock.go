package manifest

import (
	"errors"
	"fmt"
	"os"
	"syscall"
	"time"
)

// Several mm can run on one library at once: the interface open in one
// terminal, a command in another. Whoever changes the records does it in one
// turn: takes the lock, reads the records as they are now, changes them and
// writes them back. Reading is free, since a save replaces the file whole.
//
// The lock is flock(2) on library.json.lock, a file of its own, because
// library.json is replaced by every save and a lock on a replaced file locks
// nothing. The system lets go of it when the process ends, however it ends, so
// an mm that crashes cannot leave the library locked.

// lockWait is how long a change waits for another mm that is changing the
// library before it gives up. A change takes milliseconds; only an mm that
// is stopped can hold the library longer.
var lockWait = 30 * time.Second

// lockPoll is how often a change waiting for the lock looks again.
const lockPoll = 10 * time.Millisecond

// Update changes the records: it reads them as they are now, calls change on
// them and saves what it leaves, while no other mm changes them. If change
// fails, nothing is saved and its error is returned. change must be quick, and
// must not change the records through the store again, which would wait for
// itself: anything slow (a download, a lookup) is done before Update.
func (s *Store) Update(change func(*Manifest) error) error {
	return s.locked(func(m *Manifest) error {
		if err := change(m); err != nil {
			return err
		}
		return s.save(m)
	})
}

// Hold reads the records as they are now and calls f on them while no other mm
// changes them, and saves nothing. It is for what must not happen between a
// reading of the records and the end of what is done with it, like deleting the
// files that no record names.
func (s *Store) Hold(f func(*Manifest) error) error {
	return s.locked(f)
}

func (s *Store) locked(f func(*Manifest) error) error {
	unlock, err := s.lock()
	if err != nil {
		return err
	}
	defer unlock()
	m, err := s.Load()
	if err != nil {
		return err
	}
	return f(m)
}

// lock takes the lock, waiting for it up to lockWait, and returns what lets go.
func (s *Store) lock() (unlock func(), err error) {
	f, err := os.OpenFile(s.path+".lock", os.O_RDWR|os.O_CREATE, 0o644)
	if err != nil {
		return nil, err
	}
	deadline := time.Now().Add(lockWait)
	for {
		err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
		switch {
		case err == nil:
			return func() { f.Close() }, nil // closing the file lets go of the lock
		case !errors.Is(err, syscall.EWOULDBLOCK) && !errors.Is(err, syscall.EINTR):
			f.Close()
			return nil, fmt.Errorf("lock %s: %w", f.Name(), err)
		case time.Now().After(deadline):
			f.Close()
			return nil, fmt.Errorf("another mm has been changing the library for %v; try again in a moment (%s is the lock)", lockWait, f.Name())
		}
		time.Sleep(lockPoll)
	}
}
