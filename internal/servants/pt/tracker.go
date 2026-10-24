package pt

import (
	"github.com/gin-gonic/gin"
	corept "github.com/rocboss/paopao-ce/internal/core/pt"
	"github.com/rocboss/paopao-ce/internal/pt/tracker"
)

// RegisterTrackerRoutes registers the BitTorrent tracker announce and scrape endpoints.
func RegisterTrackerRoutes(e *gin.Engine, ptSrv corept.PTService) {
	tr := tracker.NewTracker(ptSrv)

	// Direct tracker paths
	e.GET("/tracker/announce", tr.HandleAnnounce)
	e.GET("/tracker/scrape", tr.HandleScrape)
	e.GET("/tracker/ws", tr.HandleWebSocket)
	e.GET("/announce", tr.HandleAnnounce)
	e.GET("/scrape", tr.HandleScrape)
}
