package core

import (
	"context"
	"sync"

	"github.com/Hhz0823/1s-ui/logger"

	sb "github.com/sagernet/sing-box"
	"github.com/sagernet/sing-box/adapter"
	_ "github.com/sagernet/sing-box/experimental/clashapi"
	_ "github.com/sagernet/sing-box/experimental/v2rayapi"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	_ "github.com/sagernet/sing-box/transport/v2rayquic"
	"github.com/sagernet/sing/service"
)

var (
	globalCtx        context.Context
	globalCtxOnce    sync.Once
	inbound_manager  adapter.InboundManager
	outbound_manager adapter.OutboundManager
	service_manager  adapter.ServiceManager
	endpoint_manager adapter.EndpointManager
	router           adapter.Router
	factory          log.Factory
)

type Core struct {
	isRunning     bool
	instance      *Box
	limitsAccess  sync.RWMutex
	inboundLimits map[string]InboundBandwidthLimit
}

// NewCore creates a lightweight handle. Full sing-box protocol registries are
// initialized lazily on first Start/GetCtx so safe-mode panel boots use less RAM.
func NewCore() *Core {
	return &Core{
		isRunning:     false,
		instance:      nil,
		inboundLimits: make(map[string]InboundBandwidthLimit),
	}
}

func (c *Core) SetInboundLimits(limits map[string]InboundBandwidthLimit) {
	copyLimits := make(map[string]InboundBandwidthLimit, len(limits))
	for tag, limit := range limits {
		copyLimits[tag] = limit
	}
	c.limitsAccess.Lock()
	c.inboundLimits = copyLimits
	instance := c.instance
	c.limitsAccess.Unlock()
	if instance != nil && instance.StatsTracker() != nil {
		instance.StatsTracker().SyncInboundLimits(copyLimits)
	}
}

func (c *Core) SetInboundLimit(tag string, limit InboundBandwidthLimit) {
	c.limitsAccess.Lock()
	c.inboundLimits[tag] = limit
	instance := c.instance
	c.limitsAccess.Unlock()
	if instance != nil && instance.StatsTracker() != nil {
		instance.StatsTracker().SetInboundLimit(tag, limit)
	}
}

func (c *Core) RemoveInboundLimit(tag string) {
	c.limitsAccess.Lock()
	delete(c.inboundLimits, tag)
	instance := c.instance
	c.limitsAccess.Unlock()
	if instance != nil && instance.StatsTracker() != nil {
		instance.StatsTracker().RemoveInboundLimit(tag)
	}
}

func (c *Core) applyInboundLimits() {
	c.limitsAccess.RLock()
	limits := make(map[string]InboundBandwidthLimit, len(c.inboundLimits))
	for tag, limit := range c.inboundLimits {
		limits[tag] = limit
	}
	instance := c.instance
	c.limitsAccess.RUnlock()
	if instance != nil && instance.StatsTracker() != nil {
		instance.StatsTracker().SyncInboundLimits(limits)
	}
}

func ensureGlobalCtx() {
	globalCtxOnce.Do(func() {
		ctx := context.Background()
		// One URL-test history shared by every group outbound and the Clash API,
		// so the panel can report the delays sing-box measured itself.
		ctx = service.ContextWithPtr(ctx, urlTestHistory)
		ctx = service.ContextWith[adapter.URLTestHistoryStorage](ctx, urlTestHistory)
		globalCtx = sb.Context(ctx, InboundRegistry(), OutboundRegistry(), EndpointRegistry(), DNSTransportRegistry(), ServiceRegistry())
	})
}

func (c *Core) GetCtx() context.Context {
	ensureGlobalCtx()
	return globalCtx
}

func (c *Core) GetInstance() *Box {
	return c.instance
}

func (c *Core) Start(sbConfig []byte) error {
	ensureGlobalCtx()

	var opt option.Options
	err := opt.UnmarshalJSONContext(globalCtx, sbConfig)
	if err != nil {
		logger.Error("Unmarshal config err:", err.Error())
		return err
	}

	c.instance, err = NewBox(Options{
		Context: globalCtx,
		Options: opt,
	})
	if err != nil {
		return err
	}
	c.applyInboundLimits()

	err = c.instance.Start()
	if err != nil {
		_ = c.instance.Close()
		c.instance = nil
		return err
	}

	globalCtx = service.ContextWith(globalCtx, c)
	inbound_manager = service.FromContext[adapter.InboundManager](globalCtx)
	outbound_manager = service.FromContext[adapter.OutboundManager](globalCtx)
	service_manager = service.FromContext[adapter.ServiceManager](globalCtx)
	endpoint_manager = service.FromContext[adapter.EndpointManager](globalCtx)
	router = service.FromContext[adapter.Router](globalCtx)

	c.isRunning = true
	return nil
}

func (c *Core) Stop() error {
	c.isRunning = false
	if c.instance == nil {
		return nil
	}
	err := c.instance.Close()
	c.instance = nil
	return err
}

func (c *Core) IsRunning() bool {
	return c.isRunning
}
