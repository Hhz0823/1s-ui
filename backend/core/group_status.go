package core

import (
	"context"
	"errors"
	"time"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/common/urltest"
	"github.com/sagernet/sing-box/option"
)

var urlTestHistory = urltest.NewHistoryStorage()

type OutboundDelay struct {
	Delay uint16
	Time  time.Time
}

// OutboundGroupStatus describes a running selector/urltest group.
type OutboundGroupStatus struct {
	Now     string
	Members []string
	Delays  map[string]OutboundDelay
}

func (c *Core) runningGroup(tag string) (adapter.OutboundGroup, error) {
	if c == nil || !c.isRunning || outbound_manager == nil {
		return nil, errors.New("sing-box is not running")
	}
	outbound, found := outbound_manager.Outbound(tag)
	if !found {
		return nil, errors.New("outbound group not found")
	}
	group, ok := outbound.(adapter.OutboundGroup)
	if !ok {
		return nil, errors.New("outbound is not a group")
	}
	return group, nil
}

// GroupStatus returns the member currently selected by a group outbound and
// the latest delay sing-box measured for each member.
func (c *Core) GroupStatus(tag string) (*OutboundGroupStatus, error) {
	group, err := c.runningGroup(tag)
	if err != nil {
		return nil, err
	}
	status := &OutboundGroupStatus{Now: group.Now(), Members: group.All(), Delays: map[string]OutboundDelay{}}
	for _, member := range status.Members {
		if history := urlTestHistory.LoadURLTestHistory(member); history != nil {
			status.Delays[member] = OutboundDelay{Delay: history.Delay, Time: history.Time}
		}
	}
	return status, nil
}

// groupChecker is implemented by sing-box urltest outbounds; unlike URLTest it
// re-tests members that were measured recently and then re-selects.
type groupChecker interface {
	CheckOutbounds()
}

// GroupCheckNow re-tests every member of a urltest group immediately and lets
// the group switch to the best one.
func (c *Core) GroupCheckNow(tag string) error {
	group, err := c.runningGroup(tag)
	if err != nil {
		return err
	}
	checker, ok := group.(groupChecker)
	if !ok {
		return errors.New("outbound group does not support URL tests")
	}
	checker.CheckOutbounds()
	return nil
}

// GroupHeal speeds up failover. sing-box drops the URL-test history of a
// member when a connection through it fails, but only re-selects on its next
// periodic test. When the active member has no history, re-test right away.
func (c *Core) GroupHeal(tag string) (bool, error) {
	group, err := c.runningGroup(tag)
	if err != nil {
		return false, err
	}
	if now := group.Now(); now != "" && urlTestHistory.LoadURLTestHistory(now) != nil {
		return false, nil
	}
	checker, ok := group.(groupChecker)
	if !ok {
		return false, nil
	}
	checker.CheckOutbounds()
	return true, nil
}

// GroupURLTest runs a URL test for the members of a urltest group whose last
// result is older than the group interval.
func (c *Core) GroupURLTest(ctx context.Context, tag string) (map[string]uint16, error) {
	group, err := c.runningGroup(tag)
	if err != nil {
		return nil, err
	}
	tester, ok := group.(adapter.URLTestGroup)
	if !ok {
		return nil, errors.New("outbound group does not support URL tests")
	}
	return tester.URLTest(ctx)
}

// ValidateOutboundJSON decodes an outbound with the same registries sing-box
// uses at start, so invalid generated configs are caught before they can stop
// the whole core from starting.
func ValidateOutboundJSON(raw []byte) error {
	ensureGlobalCtx()
	var outbound option.Outbound
	return outbound.UnmarshalJSONContext(globalCtx, raw)
}

// HasInbound reports whether the running core serves an inbound with tag.
func (c *Core) HasInbound(tag string) bool {
	if c == nil || !c.isRunning || inbound_manager == nil {
		return false
	}
	_, found := inbound_manager.Get(tag)
	return found
}

// HasOutbound reports whether the running core has an outbound or endpoint
// with tag, i.e. whether it can be measured.
func (c *Core) HasOutbound(tag string) bool {
	if c == nil || !c.isRunning {
		return false
	}
	_, err := checkDialer(tag)
	return err == nil
}
