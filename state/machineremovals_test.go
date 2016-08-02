// Copyright 2016 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package state_test

import (
	jc "github.com/juju/testing/checkers"
	gc "gopkg.in/check.v1"

	"github.com/juju/juju/state"
	"github.com/juju/juju/state/testing"
)

type MachineRemovalSuite struct {
	ConnSuite
}

var _ = gc.Suite(&MachineRemovalSuite{})

func (s *MachineRemovalSuite) TestAddingAndClearingMachineRemoval(c *gc.C) {
	m1 := s.makeDeadMachine(c)
	m2 := s.makeDeadMachine(c)

	err := m1.Remove()
	c.Assert(err, jc.ErrorIsNil)
	err = m2.Remove()
	c.Assert(err, jc.ErrorIsNil)

	removals, err := s.State.AllMachineRemovals()
	c.Assert(err, jc.ErrorIsNil)
	c.Check(getMachineIDs(removals), jc.SameContents, []string{m1.Id(), m2.Id()})

	err = s.State.ClearMachineRemovals([]string{m1.Id()})
	c.Assert(err, jc.ErrorIsNil)
	removals2, err := s.State.AllMachineRemovals()
	c.Check(getMachineIDs(removals2), jc.SameContents, []string{m2.Id()})
}

func (s *MachineRemovalSuite) TestWatchMachineRemovals(c *gc.C) {
	w, wc := s.createRemovalWatcher(c, s.State)
	wc.AssertOneChange() // Initial event.

	m1 := s.makeDeadMachine(c)
	m2 := s.makeDeadMachine(c)

	err := m1.Remove()
	c.Assert(err, jc.ErrorIsNil)
	wc.AssertOneChange()

	err = m2.Remove()
	c.Assert(err, jc.ErrorIsNil)
	wc.AssertOneChange()

	s.State.ClearMachineRemovals([]string{m1.Id(), m2.Id()})
	wc.AssertOneChange()

	testing.AssertStop(c, w)
	wc.AssertClosed()
}

func getMachineIDs(removals []*state.MachineRemoval) []string {
	result := make([]string, len(removals))
	for i, removal := range removals {
		result[i] = removal.MachineID()
	}
	return result
}

func (s *MachineRemovalSuite) createRemovalWatcher(c *gc.C, st *state.State) (
	state.NotifyWatcher, testing.NotifyWatcherC,
) {
	w := st.WatchMachineRemovals()
	s.AddCleanup(func(c *gc.C) { testing.AssertStop(c, w) })
	return w, testing.NewNotifyWatcherC(c, st, w)
}

func (s *MachineRemovalSuite) makeDeadMachine(c *gc.C) *state.Machine {
	m, err := s.State.AddMachine("xenial", state.JobHostUnits)
	c.Assert(err, jc.ErrorIsNil)
	err = m.EnsureDead()
	c.Assert(err, jc.ErrorIsNil)
	return m
}
