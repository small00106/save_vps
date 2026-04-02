package command

import (
	"encoding/json"
	"net/http"

	"github.com/cloudnest/cloudnest/internal/database/dbcore"
	"github.com/cloudnest/cloudnest/internal/database/models"
	"github.com/cloudnest/cloudnest/internal/ws"
	"github.com/gin-gonic/gin"
)

// Exec handles POST /api/nodes/:uuid/exec
func Exec(c *gin.Context) {
	nodeUUID := c.Param("uuid")

	var req struct {
		Command string `json:"command" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Create command task record
	task := models.CommandTask{
		NodeUUID: nodeUUID,
		Command:  req.Command,
		Status:   "pending",
	}
	dbcore.DB().Create(&task)

	// Send to agent via WebSocket
	params, _ := json.Marshal(map[string]interface{}{
		"task_id": task.ID,
		"command": req.Command,
	})

	hub := ws.GetHub()
	err := hub.SendToAgent(nodeUUID, &ws.RPCMessage{
		JSONRPC: "2.0",
		Method:  "master.execCommand",
		Params:  params,
	})

	if err != nil {
		task.Status = "failed"
		task.Output = "agent not connected"
		dbcore.DB().Save(&task)
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "agent not connected"})
		return
	}

	task.Status = "running"
	dbcore.DB().Save(&task)

	c.JSON(http.StatusOK, gin.H{
		"task_id": task.ID,
		"status":  "running",
	})
}

// GetTask handles GET /api/commands/:id
func GetTask(c *gin.Context) {
	id := c.Param("id")

	var task models.CommandTask
	if err := dbcore.DB().First(&task, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "task not found"})
		return
	}

	c.JSON(http.StatusOK, task)
}
