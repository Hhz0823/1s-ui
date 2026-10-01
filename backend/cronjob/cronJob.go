package cronjob

import (
	"time"

	"github.com/Hhz0823/1s-ui/logger"
	"github.com/Hhz0823/1s-ui/service"

	"github.com/robfig/cron/v3"
)

// cronParser accepts standard 5-field cron, optional leading seconds, and
// descriptors such as @daily or @every 10s.
var cronParser = cron.NewParser(
	cron.SecondOptional | cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow | cron.Descriptor,
)

type CronJob struct {
	cron *cron.Cron
}

func NewCronJob() *CronJob {
	return &CronJob{}
}

func (c *CronJob) Start(loc *time.Location, trafficAge int, statsBucketSeconds int64, globalReset string) error {
	c.cron = cron.New(cron.WithLocation(loc), cron.WithParser(cronParser))
	c.cron.Start()

	go func() {
		// Start stats job
		c.cron.AddJob("@every 10s", NewStatsJob(trafficAge > 0, statsBucketSeconds))
		// Start expiry job
		c.cron.AddJob("@every 1m", NewDepleteJob())
		// Periodic global traffic reset, only when a valid cron spec is configured
		if globalReset != "" && globalReset != "off" {
			schedule, err := cronParser.Parse(globalReset)
			if err != nil {
				logger.Warning("invalid globalReset cron spec <", globalReset, ">: ", err)
			} else {
				c.cron.AddJob(globalReset, NewResetTrafficJob(schedule))
			}
		}
		// Start deleting old stats
		if trafficAge > 0 {
			c.cron.AddJob("@daily", NewDelStatsJob(trafficAge))
		}
		// Start core if it is not running
		c.cron.AddJob("@every 5s", NewCheckCoreJob())
		// Fail SD-WAN traffic over as soon as the active exit breaks
		c.cron.AddJob("@every 5s", cron.NewChain(cron.SkipIfStillRunning(cron.DiscardLogger)).Then(NewSdwanHealJob()))
		// Store server monitor history and merge old rows
		metrics := &service.AgentService{}
		c.cron.AddJob("@every 1m", cron.NewChain(cron.SkipIfStillRunning(cron.DiscardLogger)).Then(cron.FuncJob(func() {
			if err := metrics.FlushAgentMetrics(); err != nil {
				logger.Warning("store server metrics: ", err)
			}
		})))
		c.cron.AddJob("@hourly", cron.NewChain(cron.SkipIfStillRunning(cron.DiscardLogger)).Then(cron.FuncJob(func() {
			if err := metrics.CompactAgentMetrics(); err != nil {
				logger.Warning("compact server metrics: ", err)
			}
		})))
		// Check proxy and node monitors when they are due
		proxies := &service.ProxyMonitorService{}
		c.cron.AddJob("@every 10s", cron.NewChain(cron.SkipIfStillRunning(cron.DiscardLogger)).Then(cron.FuncJob(func() {
			proxies.RunDue(time.Now())
		})))
		// Follow changes to nodes picked from a server's inbounds
		c.cron.AddJob("@every 2m", cron.NewChain(cron.SkipIfStillRunning(cron.DiscardLogger)).Then(cron.FuncJob(func() {
			proxies.RefreshNodeLinks(time.Now())
		})))
		// Proxy client: subscriptions, weekly rule files, and the dnsmasq
		// hand-off on OpenWrt following whether sing-box serves DNS.
		client := &service.ProxyClientService{}
		c.cron.AddJob("@every 10m", cron.NewChain(cron.SkipIfStillRunning(cron.DiscardLogger)).Then(cron.FuncJob(func() {
			client.UpdateDueSubscriptions(time.Now())
		})))
		c.cron.AddJob("@every 6h", cron.NewChain(cron.SkipIfStillRunning(cron.DiscardLogger)).Then(cron.FuncJob(func() {
			client.RefreshOldRuleSets(time.Now())
		})))
		c.cron.AddJob("@every 1m", cron.NewChain(cron.SkipIfStillRunning(cron.DiscardLogger)).Then(cron.FuncJob(func() {
			client.SyncDNS()
		})))
		c.cron.AddJob("@hourly", cron.NewChain(cron.SkipIfStillRunning(cron.DiscardLogger)).Then(cron.FuncJob(func() {
			if err := proxies.CleanupProxyResults(); err != nil {
				logger.Warning("clean proxy checks: ", err)
			}
		})))
		// database WAL checkpoint
		c.cron.AddJob("@every 10m", NewWALCheckpointJob())
		// Renew generated certificates; the first run waits for the core.
		renew := cron.NewChain(cron.SkipIfStillRunning(cron.DiscardLogger)).Then(NewTLSRenewJob())
		c.cron.AddJob("@every 6h", renew)
		time.AfterFunc(30*time.Second, renew.Run)
		// Keep the web UI at the panel's version
		repairUI := cron.NewChain(cron.SkipIfStillRunning(cron.DiscardLogger)).Then(NewFrontendRepairJob())
		c.cron.AddJob("@every 1h", repairUI)
		time.AfterFunc(20*time.Second, repairUI.Run)
	}()

	return nil
}

func (c *CronJob) Stop() {
	if c.cron != nil {
		c.cron.Stop()
	}
}
