package server

import (
	"embed"
	"net/http"
	"time"

	"github.com/afumu/openlink/internal/config"
	"github.com/afumu/openlink/internal/evolution"
	"github.com/afumu/openlink/internal/memory"
	"github.com/gin-gonic/gin"
)

//go:embed web
var consoleFS embed.FS

// setupEvolutionRoutes 注册自进化相关 HTTP 接口与控制台。
func (s *Server) setupEvolutionRoutes() {
	s.router.GET("/console", s.handleConsole)
	s.router.GET("/api/evolution", s.handleGetEvolution)
	s.router.POST("/api/evolution", s.handlePostEvolution)
	s.router.GET("/api/evolution/log", s.handleEvolutionLog)
	s.router.GET("/api/evolution/notify", s.handleEvolutionNotify)
	s.router.POST("/api/evolution/notify/clear", s.handleEvolutionNotifyClear)
	s.router.POST("/api/evolution/action", s.handleEvolutionAction)
	s.router.GET("/api/memory", s.handleGetMemory)
}

func (s *Server) handleConsole(c *gin.Context) {
	data, err := consoleFS.ReadFile("web/console.html")
	if err != nil {
		c.String(http.StatusInternalServerError, "console not found")
		return
	}
	c.Data(http.StatusOK, "text/html; charset=utf-8", data)
}

func (s *Server) handleGetEvolution(c *gin.Context) {
	cfg := config.Get()
	c.JSON(http.StatusOK, gin.H{
		"evolution": cfg.Evolution,
		"status":    evolution.Get().Status(),
	})
}

func (s *Server) handlePostEvolution(c *gin.Context) {
	var body struct {
		Enabled     *bool  `json:"enabled"`
		IdleMinutes *int   `json:"idle_minutes"`
		MinRounds   *int   `json:"min_rounds"`
		DreamTime   string `json:"dream_time"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	patch := &config.Settings{}
	if body.Enabled != nil {
		patch.Evolution.Enabled = *body.Enabled
	}
	if body.IdleMinutes != nil {
		patch.Evolution.IdleMinutes = *body.IdleMinutes
	}
	if body.MinRounds != nil {
		patch.Evolution.MinRounds = *body.MinRounds
	}
	if body.DreamTime != "" {
		patch.Evolution.DreamTime = body.DreamTime
	}
	updated := config.Update(patch)
	c.JSON(http.StatusOK, gin.H{"evolution": updated.Evolution})
}

func (s *Server) handleEvolutionLog(c *gin.Context) {
	records := evolution.Log().ListRecords(50)
	c.JSON(http.StatusOK, gin.H{"records": records})
}

func (s *Server) handleEvolutionNotify(c *gin.Context) {
	notes := evolution.Log().ListNotifications()
	c.JSON(http.StatusOK, gin.H{"notifications": notes})
}

func (s *Server) handleEvolutionNotifyClear(c *gin.Context) {
	evolution.Log().ClearNotifications()
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (s *Server) handleEvolutionAction(c *gin.Context) {
	var body struct {
		Action  string `json:"action"`
		Days    int    `json:"days"`
		Summary string `json:"summary"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	switch body.Action {
	case "review_now":
		c.JSON(http.StatusOK, evolution.Get().BeginReview())
	case "review_done":
		c.JSON(http.StatusOK, evolution.Get().FinishReview(body.Summary))
	case "dream_now":
		c.JSON(http.StatusOK, evolution.Get().BeginDream())
	case "dream_done":
		c.JSON(http.StatusOK, evolution.Get().FinishDream(body.Summary))
	case "enable":
		config.Update(&config.Settings{Evolution: config.EvolutionSettings{Enabled: true}})
		c.JSON(http.StatusOK, gin.H{"ok": true})
	case "disable":
		config.Update(&config.Settings{Evolution: config.EvolutionSettings{Enabled: false}})
		c.JSON(http.StatusOK, gin.H{"ok": true})
	default:
		c.JSON(400, gin.H{"error": "unknown action"})
	}
}

func (s *Server) handleGetMemory(c *gin.Context) {
	today := time.Now().Format("2006-01-02")
	c.JSON(http.StatusOK, gin.H{
		"core": memory.Get().ReadCore(),
		"daily": memory.Get().ReadDaily(""),
		"date": today,
	})
}

// startEvolutionDaemons 启动空闲监控与每日梦境定时任务。
func (s *Server) startEvolutionDaemons() {
	// 空闲监控：每 60 秒检查一次是否满足复盘条件
	go func() {
		ticker := time.NewTicker(60 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			evolution.Get().MaybeReview()
		}
	}()
	// 梦境定时：默认每天 DreamTime（如 23:55）执行一次
	go func() {
		for {
			cfg := config.Get()
			t, err := time.Parse("15:04", cfg.Evolution.DreamTime)
			if err != nil {
				t = time.Date(0, 1, 1, 23, 55, 0, 0, time.UTC)
			}
			now := time.Now()
			next := time.Date(now.Year(), now.Month(), now.Day(), t.Hour(), t.Minute(), 0, 0, now.Location())
			if !next.After(now) {
				next = next.AddDate(0, 0, 1)
			}
		d := time.Until(next)
		time.Sleep(d)
		// 仅「请求」梦境整理（提醒网页 AI 执行），语义推理由网页端 LLM 完成
		evolution.Get().RequestDream()
	}
	}()
}
