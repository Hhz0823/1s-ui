package service

import (
	"encoding/json"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Hhz0823/1s-ui/core"
	"github.com/Hhz0823/1s-ui/database"
	"github.com/Hhz0823/1s-ui/database/model"
	"github.com/Hhz0823/1s-ui/logger"
	"github.com/Hhz0823/1s-ui/util/common"
	"gorm.io/gorm"
)

var (
	LastUpdate          atomic.Int64
	corePtr             *core.Core
	xrayPtr             *core.XrayRuntime
	startCoreMu         sync.Mutex
	startCoreInProgress bool
	lastStartFailTime   time.Time
	startCooldown       = 15 * time.Second
)

type ConfigService struct {
	ClientService
	TlsService
	SettingService
	InboundService
	OutboundService
	ServicesService
	EndpointService
}

type SingBoxConfig struct {
	Log          json.RawMessage   `json:"log"`
	Dns          json.RawMessage   `json:"dns"`
	Ntp          json.RawMessage   `json:"ntp"`
	Inbounds     []json.RawMessage `json:"inbounds"`
	Outbounds    []json.RawMessage `json:"outbounds"`
	Services     []json.RawMessage `json:"services"`
	Endpoints    []json.RawMessage `json:"endpoints"`
	Route        json.RawMessage   `json:"route"`
	Experimental json.RawMessage   `json:"experimental"`
}

func NewConfigService(singCore *core.Core) *ConfigService {
	corePtr = singCore
	xrayPtr = core.NewXrayRuntime()
	return &ConfigService{}
}

func (s *ConfigService) GetConfig(data string) (*[]byte, error) {
	var err error
	if len(data) == 0 {
		data, err = s.SettingService.GetConfig()
		if err != nil {
			return nil, err
		}
	}
	singboxConfig := SingBoxConfig{}
	err = json.Unmarshal([]byte(data), &singboxConfig)
	if err != nil {
		return nil, err
	}

	singboxConfig.Inbounds, err = s.InboundService.GetAllConfig(database.GetDB())
	if err != nil {
		return nil, err
	}
	singboxConfig.Outbounds, err = s.OutboundService.GetAllConfig(database.GetDB())
	if err != nil {
		return nil, err
	}
	singboxConfig.Services, err = s.ServicesService.GetAllConfig(database.GetDB())
	if err != nil {
		return nil, err
	}
	singboxConfig.Endpoints, err = s.EndpointService.GetAllConfig(database.GetDB())
	if err != nil {
		return nil, err
	}
	rawConfig, err := json.MarshalIndent(singboxConfig, "", "  ")
	if err != nil {
		return nil, err
	}
	return &rawConfig, nil
}

func (s *ConfigService) StartCore() error {
	startCoreMu.Lock()
	if startCoreInProgress {
		startCoreMu.Unlock()
		return common.NewError("core operation already in progress")
	}
	if time.Since(lastStartFailTime) < startCooldown {
		remaining := startCooldown - time.Since(lastStartFailTime)
		logger.Info("start core cooldown ", remaining.Round(time.Second))
		startCoreMu.Unlock()
		return common.NewErrorf("core restart cooldown active: %s remaining", remaining.Round(time.Second))
	}
	startCoreInProgress = true
	startCoreMu.Unlock()
	defer func() {
		startCoreMu.Lock()
		startCoreInProgress = false
		startCoreMu.Unlock()
	}()

	if !corePtr.IsRunning() {
		logger.Info("starting core")
		rawConfig, err := s.GetConfig("")
		if err != nil {
			return err
		}
		err = corePtr.Start(*rawConfig)
		if err != nil {
			startCoreMu.Lock()
			lastStartFailTime = time.Now()
			startCoreMu.Unlock()
			logger.Error("start sing-box err:", err.Error())
			return err
		}
		logger.Info("sing-box started")
	}

	err := s.ensureXrayCore(false, false)
	if err != nil {
		startCoreMu.Lock()
		lastStartFailTime = time.Now()
		startCoreMu.Unlock()
	}
	return err
}

func (s *ConfigService) RestartCore() error {
	err := s.StopCore()
	if err != nil {
		return err
	}
	startCoreMu.Lock()
	lastStartFailTime = time.Time{}
	startCoreMu.Unlock()
	return s.StartCore()
}

func (s *ConfigService) restartSingBoxCore() error {
	startCoreMu.Lock()
	if startCoreInProgress {
		startCoreMu.Unlock()
		return common.NewError("core operation already in progress")
	}
	startCoreInProgress = true
	startCoreMu.Unlock()
	defer func() {
		startCoreMu.Lock()
		startCoreInProgress = false
		startCoreMu.Unlock()
	}()

	if corePtr != nil && corePtr.IsRunning() {
		if err := corePtr.Stop(); err != nil {
			return err
		}
	}
	rawConfig, err := s.GetConfig("")
	if err != nil {
		return err
	}
	return corePtr.Start(*rawConfig)
}

func (s *ConfigService) restartCoreWithConfig(config json.RawMessage) error {
	startCoreMu.Lock()
	if startCoreInProgress {
		startCoreMu.Unlock()
		return common.NewError("core operation already in progress")
	}
	startCoreInProgress = true
	startCoreMu.Unlock()
	defer func() {
		startCoreMu.Lock()
		startCoreInProgress = false
		startCoreMu.Unlock()
	}()

	if corePtr.IsRunning() {
		if err := corePtr.Stop(); err != nil {
			logger.Error("restart sing-box err (stop):", err.Error())
			return err
		}
	}
	rawConfig, err := s.GetConfig(string(config))
	if err != nil {
		logger.Error("restart sing-box err (get config):", err.Error())
		return err
	}
	if err := corePtr.Start(*rawConfig); err != nil {
		logger.Error("restart sing-box err (start):", err.Error())
		return err
	}
	logger.Info("sing-box restarted with new config")
	return s.ensureXrayCore(false, false)
}

func (s *ConfigService) StopCore() error {
	var result error
	if xrayPtr != nil {
		if err := xrayPtr.Stop(); err != nil {
			logger.Warning("stop xray err:", err)
			result = err
		}
	}
	err := corePtr.Stop()
	if err != nil {
		result = err
	}
	logger.Info("sing-box stopped")
	return result
}

func (s *ConfigService) RestartXrayCoreIfNeeded() error {
	return s.restartXrayCore(false)
}

func (s *ConfigService) RestartXrayCore() error {
	return s.restartXrayCore(true)
}

func (s *ConfigService) restartXrayCore(manual bool) error {
	startCoreMu.Lock()
	if startCoreInProgress {
		startCoreMu.Unlock()
		return common.NewError("core operation already in progress")
	}
	startCoreInProgress = true
	startCoreMu.Unlock()
	defer func() {
		startCoreMu.Lock()
		startCoreInProgress = false
		startCoreMu.Unlock()
	}()
	return s.ensureXrayCore(true, manual)
}

func (s *ConfigService) StartXrayCoreIfNeeded() error {
	startCoreMu.Lock()
	if startCoreInProgress {
		startCoreMu.Unlock()
		return common.NewError("core operation already in progress")
	}
	startCoreInProgress = true
	startCoreMu.Unlock()
	defer func() {
		startCoreMu.Lock()
		startCoreInProgress = false
		startCoreMu.Unlock()
	}()
	return s.ensureXrayCore(false, false)
}

func (s *ConfigService) ensureXrayCore(restart bool, manual bool) error {
	if xrayPtr == nil {
		xrayPtr = core.NewXrayRuntime()
	}
	hasXray, err := s.HasXrayInbounds()
	if err != nil {
		return err
	}
	if !hasXray {
		if xrayPtr.IsRunning() {
			return xrayPtr.Stop()
		}
		if manual {
			return common.NewError("no Xray-core inbound configured; create an inbound with Core = Xray-core first")
		}
		return nil
	}

	rawConfig, err := s.GetXrayConfig()
	if err != nil {
		return err
	}
	if restart {
		return xrayPtr.Restart(*rawConfig)
	}
	if !xrayPtr.IsRunning() {
		return xrayPtr.Start(*rawConfig)
	}
	return nil
}

func (s *ConfigService) CheckOutbound(tag string, link string) core.CheckOutboundResult {
	if tag == "" {
		return core.CheckOutboundResult{Error: "missing query parameter: tag"}
	}
	if corePtr == nil || !corePtr.IsRunning() {
		return core.CheckOutboundResult{Error: "core not running"}
	}
	return core.CheckOutbound(corePtr.GetCtx(), tag, link)
}

func (s *ConfigService) CheckWarp(tag string, link string) core.CheckWarpResult {
	if tag == "" {
		return core.CheckWarpResult{Error: "missing query parameter: tag"}
	}
	if corePtr == nil || !corePtr.IsRunning() {
		return core.CheckWarpResult{Error: "core not running"}
	}
	return core.CheckWarp(corePtr.GetCtx(), tag, link)
}

func inboundIDsContainXray(tx *gorm.DB, ids []uint) (bool, error) {
	if len(ids) == 0 {
		return false, nil
	}
	var count int64
	err := tx.Model(model.Inbound{}).
		Where("id IN ? AND core_type = ?", ids, model.CoreTypeXray).
		Count(&count).Error
	return count > 0, err
}

func inboundChangeAffectsXray(tx *gorm.DB, act string, data json.RawMessage) (bool, error) {
	switch act {
	case "new", "edit":
		var inbound model.Inbound
		if err := inbound.UnmarshalJSON(data); err != nil {
			return false, err
		}
		if inbound.RuntimeCore() == model.CoreTypeXray {
			return true, nil
		}
		if act == "new" {
			return false, nil
		}
		var oldInbound model.Inbound
		err := tx.Model(model.Inbound{}).Select("core_type").Where("id = ?", inbound.Id).First(&oldInbound).Error
		return oldInbound.RuntimeCore() == model.CoreTypeXray, err
	case "del":
		var tag string
		if err := json.Unmarshal(data, &tag); err != nil {
			return false, err
		}
		var inbound model.Inbound
		err := tx.Model(model.Inbound{}).Select("core_type").Where("tag = ?", tag).First(&inbound).Error
		return inbound.RuntimeCore() == model.CoreTypeXray, err
	default:
		return false, nil
	}
}

func tlsChangeAffectsXray(tx *gorm.DB, act string, data json.RawMessage) (bool, error) {
	if act != "new" && act != "edit" {
		return false, nil
	}
	var tls model.Tls
	if err := json.Unmarshal(data, &tls); err != nil {
		return false, err
	}
	if tls.Id == 0 {
		return false, nil
	}
	var count int64
	err := tx.Model(model.Inbound{}).
		Where("tls_id = ? AND core_type = ?", tls.Id, model.CoreTypeXray).
		Count(&count).Error
	return count > 0, err
}

func (s *ConfigService) Save(obj string, act string, data json.RawMessage, initUsers string, loginUser string, hostname string) (objs []string, err error) {
	objs = []string{obj}
	restartXray := false
	restartWithConfig := false
	runtimeMayHaveChanged := false
	var configData json.RawMessage

	db := database.GetDB()
	tx := db.Begin()
	if tx.Error != nil {
		return nil, tx.Error
	}
	committed := false
	defer func() {
		if !committed {
			if rollbackErr := tx.Rollback().Error; rollbackErr != nil && err == nil {
				err = rollbackErr
			}
			if err != nil && runtimeMayHaveChanged && corePtr != nil && corePtr.IsRunning() {
				if restoreErr := s.restartSingBoxCore(); restoreErr != nil {
					logger.Error("restore core after failed save:", restoreErr)
					err = common.NewErrorf("%v; failed to restore core: %v", err, restoreErr)
				}
			}
		}
	}()

	switch obj {
	case "clients":
		runtimeMayHaveChanged = true
		var inboundIds []uint
		inboundIds, err = s.ClientService.Save(tx, act, data, hostname)
		if err == nil && len(inboundIds) > 0 {
			objs = append(objs, "inbounds")
			err = s.InboundService.RestartInbounds(tx, inboundIds)
			if err != nil {
				return nil, common.NewErrorf("failed to update users for inbounds: %v", err)
			}
			restartXray, err = inboundIDsContainXray(tx, inboundIds)
		}
	case "tls":
		runtimeMayHaveChanged = true
		err = s.TlsService.Save(tx, act, data, hostname)
		if err == nil {
			restartXray, err = tlsChangeAffectsXray(tx, act, data)
		}
		objs = append(objs, "clients", "inbounds")
	case "inbounds":
		runtimeMayHaveChanged = true
		restartXray, err = inboundChangeAffectsXray(tx, act, data)
		if err == nil {
			err = s.InboundService.Save(tx, act, data, initUsers, hostname)
		}
		objs = append(objs, "clients")
	case "outbounds":
		runtimeMayHaveChanged = true
		err = s.OutboundService.Save(tx, act, data)
	case "services":
		runtimeMayHaveChanged = true
		err = s.ServicesService.Save(tx, act, data)
	case "endpoints":
		runtimeMayHaveChanged = true
		err = s.EndpointService.Save(tx, act, data)
	case "config":
		err = s.SettingService.SaveConfig(tx, data)
		if err != nil {
			return nil, err
		}
		configData = make(json.RawMessage, len(data))
		copy(configData, data)
		restartWithConfig = true
	case "settings":
		err = s.SettingService.Save(tx, data)
	default:
		return nil, common.NewError("unknown object: ", obj)
	}
	if err != nil {
		return nil, err
	}
	if restartXray {
		if err = s.validateXrayConfig(tx); err != nil {
			return nil, err
		}
	}

	dt := time.Now().Unix()
	err = tx.Create(&model.Changes{
		DateTime: dt,
		Actor:    loginUser,
		Key:      obj,
		Action:   act,
		Obj:      data,
	}).Error
	if err != nil {
		return nil, err
	}

	if err = tx.Commit().Error; err != nil {
		return nil, err
	}
	committed = true
	LastUpdate.Store(time.Now().UnixMilli())

	if restartWithConfig {
		err = s.restartCoreWithConfig(configData)
	} else if corePtr != nil && !corePtr.IsRunning() {
		err = s.StartCore()
	} else if restartXray {
		err = s.RestartXrayCoreIfNeeded()
	}
	if err != nil {
		return nil, common.NewErrorf("configuration saved, but core update failed: %v", err)
	}
	return objs, nil
}

func (s *ConfigService) CheckChanges(lu string) (bool, error) {
	if lu == "" {
		return true, nil
	}
	intLu, err := strconv.ParseInt(lu, 10, 64)
	if err != nil {
		return false, err
	}
	lastUpdate := LastUpdate.Load()
	if lastUpdate == 0 {
		db := database.GetDB()
		var count int64
		changeTime := intLu
		if changeTime > 1_000_000_000_000 {
			changeTime /= 1000
		}
		err = db.Model(model.Changes{}).Where("date_time > ?", changeTime).Count(&count).Error
		if err == nil {
			LastUpdate.Store(time.Now().UnixMilli())
		}
		return count > 0, err
	}
	return lastUpdate > intLu, nil
}

func (s *ConfigService) GetChanges(actor string, chngKey string, count string) []model.Changes {
	c, _ := strconv.Atoi(count)
	db := database.GetDB()
	query := db.Model(model.Changes{}).Where("id > ?", 0)
	if len(actor) > 0 {
		query = query.Where("actor = ?", actor)
	}
	if len(chngKey) > 0 {
		query = query.Where("key = ?", chngKey)
	}
	if c > 0 {
		query = query.Limit(c)
	}
	var chngs []model.Changes
	err := query.Order("id desc").Scan(&chngs).Error
	if err != nil {
		logger.Warning(err)
	}
	return chngs
}
