package web

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/Hhz0823/1s-ui/api"
	"github.com/Hhz0823/1s-ui/config"
	"github.com/Hhz0823/1s-ui/logger"
	"github.com/Hhz0823/1s-ui/service"

	"github.com/gin-contrib/gzip"
	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
)

type Server struct {
	httpServer        *http.Server
	listener          net.Listener
	controlServer     *http.Server
	controlListener   net.Listener
	controlSocketPath string
	ctx               context.Context
	cancel            context.CancelFunc
	settingService    service.SettingService
}

func NewServer() *Server {
	ctx, cancel := context.WithCancel(context.Background())
	return &Server{ctx: ctx, cancel: cancel}
}

func (s *Server) initRouter() (*gin.Engine, error) {
	if config.IsDebug() {
		gin.SetMode(gin.DebugMode)
	} else {
		gin.DefaultWriter = io.Discard
		gin.DefaultErrorWriter = io.Discard
		gin.SetMode(gin.ReleaseMode)
	}

	webPath, err := s.settingService.GetWebPath()
	if err != nil {
		return nil, err
	}
	secret, err := s.settingService.GetSecret()
	if err != nil {
		return nil, err
	}
	policy, err := api.NewOriginPolicyFromEnv()
	if err != nil {
		return nil, err
	}

	apiPaths := routePaths(webPath, "api")
	apiv2Paths := routePaths(webPath, "apiv2")
	agentPaths := routePaths(webPath, "agent/v1")
	prefixes := append(append(append([]string{}, apiPaths...), apiv2Paths...), agentPaths...)

	engine := gin.New()
	engine.Use(gin.Recovery())
	engine.Use(policy.CORSMiddleware(prefixes))
	engine.Use(gzip.Gzip(gzip.DefaultCompression))
	engine.Use(sessions.Sessions("s-ui", cookie.NewStore(secret)))

	apiv2 := api.NewAPIv2Handler(engine.Group(apiv2Paths[0]))
	for _, route := range apiv2Paths[1:] {
		apiv2.Register(engine.Group(route))
	}
	for _, route := range apiPaths {
		api.NewAPIHandler(engine.Group(route), apiv2, policy)
	}
	for _, route := range agentPaths {
		api.NewAgentHandler(engine.Group(route))
	}

	engine.NoRoute(func(c *gin.Context) {
		c.JSON(http.StatusNotFound, api.Msg{Success: false, Msg: "not found"})
	})
	return engine, nil
}

func routePaths(webPath, suffix string) []string {
	canonical := "/" + strings.Trim(suffix, "/")
	legacy := strings.TrimSuffix(webPath, "/") + canonical
	if legacy == canonical {
		return []string{canonical}
	}
	return []string{canonical, legacy}
}

func (s *Server) Start() (err error) {
	defer func() {
		if err != nil {
			_ = s.Stop()
		}
	}()

	engine, err := s.initRouter()
	if err != nil {
		return err
	}
	port, err := config.GetAPIPort()
	if err != nil {
		return err
	}
	listenAddr := net.JoinHostPort(config.GetAPIListen(), strconv.Itoa(port))
	listener, err := net.Listen("tcp", listenAddr)
	if err != nil {
		return err
	}
	s.listener = listener
	s.httpServer = &http.Server{
		Handler:           engine,
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	if err := s.startControlSocket(); err != nil {
		logger.Warning("local managed-node control is unavailable: ", err)
	}

	logger.Info("API server run http on ", listener.Addr())
	go func() {
		if serveErr := s.httpServer.Serve(listener); serveErr != nil && !errors.Is(serveErr, http.ErrServerClosed) {
			logger.Error("API server stopped: ", serveErr)
		}
	}()
	return nil
}

func (s *Server) Stop() error {
	var err error
	controlErr := s.stopControlSocket()
	if s.httpServer != nil {
		shutdownCtx, cancelShutdown := context.WithTimeout(context.Background(), 30*time.Second)
		err = s.httpServer.Shutdown(shutdownCtx)
		cancelShutdown()
		if err != nil && s.listener != nil {
			_ = s.listener.Close()
		}
	} else if s.listener != nil {
		err = s.listener.Close()
	}
	s.cancel()
	if err == nil {
		err = controlErr
	}
	return err
}

func (s *Server) GetCtx() context.Context {
	return s.ctx
}
