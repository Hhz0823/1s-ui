package service

import (
	"encoding/json"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/Hhz0823/1s-ui/config"
	"github.com/Hhz0823/1s-ui/database"
	"github.com/Hhz0823/1s-ui/database/model"
	"github.com/Hhz0823/1s-ui/logger"
	"github.com/Hhz0823/1s-ui/util/common"

	"gorm.io/gorm"
)

var defaultConfig = `{
  "log": {
    "level": "info"
  },
  "dns": {
    "servers": [],
    "rules": []
  },
  "route": {
    "rules": [
		  {
        "action": "sniff"
      },
      {
        "protocol": [
          "dns"
        ],
        "action": "hijack-dns"
      }
    ]
  },
  "experimental": {}
}`

var defaultValueMap = map[string]string{
	"webListen":          "",
	"webDomain":          "",
	"webPort":            "2095",
	"secret":             common.Random(32),
	"webCertFile":        "",
	"webKeyFile":         "",
	"webPath":            "/app/",
	"webURI":             "",
	"sessionMaxAge":      "0",
	"trafficAge":         "30",
	"statsBucketSeconds": "60",
	"timeLocation":       "Asia/Shanghai",
	"subListen":          "",
	"subPort":            "2096",
	"subPath":            "/sub/",
	"subDomain":          "",
	"subCertFile":        "",
	"subKeyFile":         "",
	"subUpdates":         "12",
	"subEncode":          "true",
	"subShowInfo":        "false",
	"subURI":             "",
	"subJsonExt":         "",
	"subClashExt":        "",
	"subClashNoDefGrp":   "false",
	"subClashSprtAll":    "false",
	"globalReset":        "",
	"globalResetLast":    "0",
	"congestionAlgo":     "",
	"qdisc":              "",
	"githubMirror":       "",
	"downloadLine":       downloadLineAuto,
	"config":             defaultConfig,
	"version":            config.GetVersion(),
	agentEnrollmentKey:   "",
	monitorKeyHash:       "",
	controllerModeKey:    controllerModeAuto,
}

type SettingService struct {
}

const frontendApplyMarker = ".frontend_apply_required"

var frontendEntrySettingKeys = map[string]struct{}{
	"webListen": {},
	"webPort":   {},
	"webPath":   {},
	"webDomain": {},
}

func (s *SettingService) GetAllSetting() (*map[string]string, error) {
	db := database.GetDB()
	settings := make([]*model.Setting, 0)
	err := db.Model(model.Setting{}).Find(&settings).Error
	if err != nil {
		return nil, err
	}
	allSetting := map[string]string{}

	for _, setting := range settings {
		allSetting[setting.Key] = setting.Value
	}

	for key, defaultValue := range defaultValueMap {
		if _, exists := allSetting[key]; !exists {
			err = s.saveSetting(key, defaultValue)
			if err != nil {
				return nil, err
			}
			allSetting[key] = defaultValue
		}
	}

	// Due to security principles
	delete(allSetting, "secret")
	delete(allSetting, "config")
	delete(allSetting, "version")
	delete(allSetting, "globalResetLast")
	delete(allSetting, agentEnrollmentKey)
	delete(allSetting, monitorKeyHash)
	delete(allSetting, controllerModeKey)
	delete(allSetting, sdwanConfigKey)
	for key, value := range s.GetDeploymentStatus() {
		allSetting[key] = value
	}

	return &allSetting, nil
}

func (s *SettingService) GetDeploymentStatus() map[string]string {
	port, err := config.GetAPIPort()
	apiListen := "invalid"
	if err == nil {
		apiListen = net.JoinHostPort(config.GetAPIListen(), strconv.Itoa(port))
	}
	pending := FrontendEntryApplyRequired()
	message := "frontend entry is applied by the managed nginx gateway"
	if pending {
		message = "frontend entry settings changed; apply the managed nginx frontend gateway"
	}
	return map[string]string{
		"deploymentMode":            "split",
		"apiListen":                 apiListen,
		"frontendEntryManagedBy":    "nginx",
		"frontendApplyRequired":     strconv.FormatBool(pending),
		"frontendApplyMessage":      message,
		"frontendGatewayConfigPath": "/etc/nginx/sites-available/s-ui-frontend.conf",
		"frontendRuntimeConfigPath": frontendRuntimeConfigPath,
	}
}

func frontendApplyMarkerPath() string {
	return filepath.Join(config.GetDBFolderPath(), frontendApplyMarker)
}

func FrontendEntryApplyRequired() bool {
	_, err := os.Stat(frontendApplyMarkerPath())
	return err == nil
}

func MarkFrontendEntryApplyRequired() error {
	if err := os.MkdirAll(config.GetDBFolderPath(), 0740); err != nil {
		return err
	}
	return os.WriteFile(frontendApplyMarkerPath(), []byte("apply nginx frontend gateway\n"), 0600)
}

func ClearFrontendEntryApplyRequired() error {
	err := os.Remove(frontendApplyMarkerPath())
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

func frontendEntrySettingsChanged(data json.RawMessage) (bool, error) {
	var settings map[string]json.RawMessage
	if err := json.Unmarshal(data, &settings); err != nil {
		return false, err
	}
	for key := range settings {
		if _, ok := frontendEntrySettingKeys[key]; ok {
			return true, nil
		}
	}
	return false, nil
}

func (s *SettingService) ResetSettings() error {
	db := database.GetDB()
	return db.Where("1 = 1").Delete(model.Setting{}).Error
}

func (s *SettingService) getSetting(key string) (*model.Setting, error) {
	db := database.GetDB()
	setting := &model.Setting{}
	err := db.Model(model.Setting{}).Where("key = ?", key).First(setting).Error
	if err != nil {
		return nil, err
	}
	return setting, nil
}

func (s *SettingService) getString(key string) (string, error) {
	setting, err := s.getSetting(key)
	if database.IsNotFound(err) {
		value, ok := defaultValueMap[key]
		if !ok {
			return "", common.NewErrorf("key <%v> not in defaultValueMap", key)
		}
		return value, nil
	} else if err != nil {
		return "", err
	}
	return setting.Value, nil
}

func (s *SettingService) saveSetting(key string, value string) error {
	setting, err := s.getSetting(key)
	db := database.GetDB()
	if database.IsNotFound(err) {
		return db.Create(&model.Setting{
			Key:   key,
			Value: value,
		}).Error
	} else if err != nil {
		return err
	}
	setting.Key = key
	setting.Value = value
	return db.Save(setting).Error
}

func (s *SettingService) setString(key string, value string) error {
	return s.saveSetting(key, value)
}

func (s *SettingService) getBool(key string) (bool, error) {
	str, err := s.getString(key)
	if err != nil {
		return false, err
	}
	return strconv.ParseBool(str)
}

// func (s *SettingService) setBool(key string, value bool) error {
// 	return s.setString(key, strconv.FormatBool(value))
// }

func (s *SettingService) getInt(key string) (int, error) {
	str, err := s.getString(key)
	if err != nil {
		return 0, err
	}
	return strconv.Atoi(str)
}

func (s *SettingService) setInt(key string, value int) error {
	return s.setString(key, strconv.Itoa(value))
}
func (s *SettingService) GetListen() (string, error) {
	return s.getString("webListen")
}

func (s *SettingService) GetWebDomain() (string, error) {
	return s.getString("webDomain")
}

func (s *SettingService) GetPort() (int, error) {
	return s.getInt("webPort")
}

func (s *SettingService) SetPort(port int) error {
	if port < 1 || port > 65535 {
		return common.NewError("panel port must be between 1 and 65535")
	}
	return s.setInt("webPort", port)
}

func (s *SettingService) SetWebListen(listen string) error {
	return s.setString("webListen", strings.TrimSpace(listen))
}

func (s *SettingService) SetWebDomain(domain string) error {
	return s.setString("webDomain", strings.TrimSpace(domain))
}

func (s *SettingService) SetWebURI(uri string) error {
	return s.setString("webURI", strings.TrimSpace(uri))
}

func (s *SettingService) GetCertFile() (string, error) {
	return s.getString("webCertFile")
}

func (s *SettingService) GetKeyFile() (string, error) {
	return s.getString("webKeyFile")
}

func (s *SettingService) GetWebPath() (string, error) {
	webPath, err := s.getString("webPath")
	if err != nil {
		return "", err
	}
	if !strings.HasPrefix(webPath, "/") {
		webPath = "/" + webPath
	}
	if !strings.HasSuffix(webPath, "/") {
		webPath += "/"
	}
	return webPath, nil
}

func (s *SettingService) SetWebPath(webPath string) error {
	webPath, err := validateFrontendPanelPath(webPath)
	if err != nil {
		return err
	}
	return s.setString("webPath", webPath)
}

func (s *SettingService) GetSecret() ([]byte, error) {
	secret, err := s.getString("secret")
	if secret == defaultValueMap["secret"] {
		err := s.saveSetting("secret", secret)
		if err != nil {
			logger.Warning("save secret failed:", err)
		}
	}
	return []byte(secret), err
}

func (s *SettingService) GetSessionMaxAge() (int, error) {
	return s.getInt("sessionMaxAge")
}

func (s *SettingService) GetTrafficAge() (int, error) {
	return s.getInt("trafficAge")
}

func (s *SettingService) GetStatsBucketSeconds() (int64, error) {
	v, err := s.getInt("statsBucketSeconds")
	if err != nil {
		return 0, err
	}
	if v < 1 {
		def, _ := strconv.Atoi(defaultValueMap["statsBucketSeconds"])
		return int64(def), nil
	}
	return int64(v), nil
}

func (s *SettingService) GetTimeLocation() (*time.Location, error) {
	l, err := s.getString("timeLocation")
	if err != nil {
		return nil, err
	}
	if runtime.GOOS == "windows" {
		l = "Local"
	}
	location, err := time.LoadLocation(l)
	if err != nil {
		defaultLocation := defaultValueMap["timeLocation"]
		logger.Errorf("location <%v> not exist, using default location: %v", l, defaultLocation)
		return time.LoadLocation(defaultLocation)
	}
	return location, nil
}

func (s *SettingService) GetSubListen() (string, error) {
	return s.getString("subListen")
}

func (s *SettingService) GetSubPort() (int, error) {
	return s.getInt("subPort")
}

func (s *SettingService) SetSubPort(subPort int) error {
	return s.setInt("subPort", subPort)
}

func (s *SettingService) GetSubPath() (string, error) {
	subPath, err := s.getString("subPath")
	if err != nil {
		return "", err
	}
	if !strings.HasPrefix(subPath, "/") {
		subPath = "/" + subPath
	}
	if !strings.HasSuffix(subPath, "/") {
		subPath += "/"
	}
	return subPath, nil
}

func (s *SettingService) SetSubPath(subPath string) error {
	if !strings.HasPrefix(subPath, "/") {
		subPath = "/" + subPath
	}
	if !strings.HasSuffix(subPath, "/") {
		subPath += "/"
	}
	return s.setString("subPath", subPath)
}

func (s *SettingService) GetSubDomain() (string, error) {
	return s.getString("subDomain")
}

func (s *SettingService) GetSubCertFile() (string, error) {
	return s.getString("subCertFile")
}

func (s *SettingService) GetSubKeyFile() (string, error) {
	return s.getString("subKeyFile")
}

func (s *SettingService) GetSubUpdates() (int, error) {
	return s.getInt("subUpdates")
}

func (s *SettingService) GetSubEncode() (bool, error) {
	return s.getBool("subEncode")
}

func (s *SettingService) GetSubShowInfo() (bool, error) {
	return s.getBool("subShowInfo")
}

func (s *SettingService) GetSubURI() (string, error) {
	return s.getString("subURI")
}

func (s *SettingService) GetGlobalReset() (string, error) {
	return s.getString("globalReset")
}

func (s *SettingService) GetGlobalResetLast() (int64, error) {
	str, err := s.getString("globalResetLast")
	if err != nil {
		return 0, err
	}
	return strconv.ParseInt(str, 10, 64)
}

func (s *SettingService) SetGlobalResetLast(value int64) error {
	return s.setString("globalResetLast", strconv.FormatInt(value, 10))
}

func (s *SettingService) GetFinalSubURI(host string) (string, error) {
	allSetting, err := s.GetAllSetting()
	if err != nil {
		return "", err
	}
	SubURI := (*allSetting)["subURI"]
	if SubURI != "" {
		return SubURI, nil
	}
	protocol := "http"
	if (*allSetting)["subKeyFile"] != "" && (*allSetting)["subCertFile"] != "" {
		protocol = "https"
	}
	if (*allSetting)["subDomain"] != "" {
		host = (*allSetting)["subDomain"]
	}
	port := ":" + (*allSetting)["subPort"]
	if (port == "80" && protocol == "http") || (port == "443" && protocol == "https") {
		port = ""
	}
	return protocol + "://" + host + port + (*allSetting)["subPath"], nil
}

func (s *SettingService) GetConfig() (string, error) {
	return s.getString("config")
}

func (s *SettingService) SetConfig(config string) error {
	return s.setString("config", config)
}

func (s *SettingService) SaveConfig(tx *gorm.DB, config json.RawMessage) error {
	configs, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}
	return tx.Model(model.Setting{}).Where("key = ?", "config").Update("value", string(configs)).Error
}

func (s *SettingService) Save(tx *gorm.DB, data json.RawMessage) error {
	var err error
	var settings map[string]string
	err = json.Unmarshal(data, &settings)
	if err != nil {
		return err
	}
	for key, obj := range settings {
		if strings.HasPrefix(key, "deployment") || strings.HasPrefix(key, "frontendApply") ||
			key == "apiListen" || key == "frontendEntryManagedBy" || key == "frontendGatewayConfigPath" || key == "frontendRuntimeConfigPath" ||
			key == controllerModeKey || key == agentEnrollmentKey || key == monitorKeyHash || key == sdwanConfigKey {
			continue
		}
		// Secure file existence check
		if obj != "" && (key == "webCertFile" ||
			key == "webKeyFile" ||
			key == "subCertFile" ||
			key == "subKeyFile") {
			err = s.fileExists(obj)
			if err != nil {
				return common.NewError(" -> ", obj, " is not exists")
			}
		}

		// Correct Pathes start and ends with `/`
		if key == "webPath" {
			obj, err = validateFrontendPanelPath(obj)
			if err != nil {
				return err
			}
		} else if key == "subPath" {
			if !strings.HasPrefix(obj, "/") {
				obj = "/" + obj
			}
			if !strings.HasSuffix(obj, "/") {
				obj += "/"
			}
		}
		if key == "githubMirror" {
			obj, err = normalizeGitHubMirror(obj)
			if err != nil {
				return err
			}
		}
		if key == "downloadLine" {
			obj, err = normalizeDownloadLine(obj)
			if err != nil {
				return err
			}
		}
		if key == "webPort" {
			port, parseErr := strconv.Atoi(obj)
			if parseErr != nil || port < 1 || port > 65535 {
				return common.NewError("panel port must be between 1 and 65535")
			}
		}

		// Delete all stats if it is set to 0
		if key == "trafficAge" && obj == "0" {
			err = tx.Where("id > 0").Delete(model.Stats{}).Error
			if err != nil {
				return err
			}
		}
		err = tx.Model(model.Setting{}).Where("key = ?", key).Update("value", obj).Error
		if err != nil {
			return err
		}
	}
	return err
}

// GetGitHubMirror is the mirror the administrator set for GitHub downloads,
// as a prefix to put before github.com URLs, or "".
func (s *SettingService) GetGitHubMirror() (string, error) {
	value, err := s.getString("githubMirror")
	if err != nil {
		return "", err
	}
	return normalizeGitHubMirror(value)
}

// normalizeGitHubMirror accepts an http(s) prefix such as
// "https://ghfast.top" and returns it ending in "/", ready to put before a
// github.com URL.
func normalizeGitHubMirror(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", nil
	}
	parsed, err := url.Parse(value)
	if err != nil || (parsed.Scheme != "https" && parsed.Scheme != "http") || parsed.Host == "" ||
		parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return "", common.NewError("GitHub mirror must be an http(s) address such as https://ghfast.top/")
	}
	if !strings.HasSuffix(value, "/") {
		value += "/"
	}
	return value, nil
}

// Download lines for panel updates and Xray-core installs:
//   - auto: GitHub first; a mirror takes over when GitHub fails or is too slow.
//   - github: GitHub and the administrator's mirror only.
//   - cn: the mirrors first (for servers in mainland China), GitHub last.
const (
	downloadLineAuto   = "auto"
	downloadLineGitHub = "github"
	downloadLineCN     = "cn"
)

// GetDownloadLine is the download line chosen under Settings.
func (s *SettingService) GetDownloadLine() (string, error) {
	value, err := s.getString("downloadLine")
	if err != nil {
		return downloadLineAuto, err
	}
	return normalizeDownloadLine(value)
}

func normalizeDownloadLine(value string) (string, error) {
	switch value = strings.ToLower(strings.TrimSpace(value)); value {
	case "":
		return downloadLineAuto, nil
	case downloadLineAuto, downloadLineGitHub, downloadLineCN:
		return value, nil
	}
	return downloadLineAuto, common.NewError("download line must be auto, github or cn")
}

func (s *SettingService) GetSubJsonExt() (string, error) {
	return s.getString("subJsonExt")
}

func (s *SettingService) GetSubClashExt() (string, error) {
	return s.getString("subClashExt")
}

func (s *SettingService) GetSubClashNoDefGrp() (bool, error) {
	return s.getBool("subClashNoDefGrp")
}

func (s *SettingService) GetSubClashSprtAll() (bool, error) {
	return s.getBool("subClashSprtAll")
}

func (s *SettingService) fileExists(path string) error {
	_, err := os.Stat(path)
	return err
}
