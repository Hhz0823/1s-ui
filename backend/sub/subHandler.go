package sub

import (
	"encoding/base64"
	"regexp"

	"github.com/Hhz0823/1s-ui/logger"
	"github.com/Hhz0823/1s-ui/service"

	"github.com/gin-gonic/gin"
)

type SubHandler struct {
	service.SettingService
	SubService
	JsonService
	ClashService
}

func NewSubHandler(g *gin.RouterGroup) {
	a := &SubHandler{}
	a.initRouter(g)
}

func (s *SubHandler) initRouter(g *gin.RouterGroup) {
	g.GET("/:subid", s.subs)
	g.HEAD("/:subid", s.subHeaders)
}

func (s *SubHandler) subs(c *gin.Context) {
	var headers []string
	var result *string
	var err error
	subId := c.Param("subid")
	format, isFormat := c.GetQuery("format")
	// The sing-box apps only import sing-box configs, so the plain link
	// serves them JSON instead of the base64 link list.
	if !isFormat && isSingBoxApp(c.GetHeader("User-Agent")) {
		format, isFormat = "json", true
	}
	if isFormat {
		switch format {
		case "json":
			result, headers, err = s.JsonService.GetJson(subId, format)
		case "clash":
			result, headers, err = s.ClashService.GetClash(subId)
		}
		if err != nil || result == nil {
			logger.Error(err)
			c.String(400, "Error!")
			return
		}
	} else {
		result, headers, err = s.SubService.GetSubs(subId)
		if err != nil || result == nil {
			logger.Error(err)
			c.String(400, "Error!")
			return
		}
	}

	s.addHeaders(c, headers)

	if format == "json" {
		c.Data(200, "application/json; charset=utf-8", []byte(*result))
		return
	}
	c.String(200, *result)
}

// singBoxAppAgent matches the official sing-box apps: SFA (Android), SFI,
// SFM and SFT (Apple), SFW and SFL (Windows and Linux), as in
// "SFA/1.12.0 (590; sing-box 1.12.0)" or "SFW (sing-box 1.15.0; language zh_CN)".
// Other clients built on sing-box (Hiddify, NekoBox) import link lists and
// keep getting them.
var singBoxAppAgent = regexp.MustCompile(`^SF[A-Z][/ (].*\bsing-box\b`)

func isSingBoxApp(userAgent string) bool {
	return singBoxAppAgent.MatchString(userAgent)
}

func (s *SubHandler) subHeaders(c *gin.Context) {
	subId := c.Param("subid")
	client, err := s.SubService.getClientBySubId(subId)
	if err != nil {
		logger.Error(err)
		c.String(400, "Error!")
		return
	}

	headers := s.SubService.getClientHeaders(client)
	s.addHeaders(c, headers)

	c.Status(200)
}

func (s *SubHandler) addHeaders(c *gin.Context, headers []string) {
	c.Writer.Header().Set("Subscription-Userinfo", headers[0])
	c.Writer.Header().Set("Profile-Update-Interval", headers[1])
	c.Writer.Header().Set("Profile-Title", profileTitleHeader(headers[2]))
}

// profileTitleHeader keeps the header ASCII-only. Clash/mihomo based clients
// decode the "base64:" prefix; raw UTF-8 names confuse strict HTTP stacks.
func profileTitleHeader(title string) string {
	for _, r := range title {
		if r < 0x20 || r > 0x7e {
			return "base64:" + base64.StdEncoding.EncodeToString([]byte(title))
		}
	}
	return title
}
