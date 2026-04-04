package ping

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/cloudnest/cloudnest/internal/database/dbcore"
	"github.com/cloudnest/cloudnest/internal/database/models"
	"github.com/cloudnest/cloudnest/internal/ws"
	"github.com/gin-gonic/gin"
)

// ListTasks handles GET /api/ping/tasks
func ListTasks(c *gin.Context) {
	var tasks []models.PingTask
	dbcore.DB().Find(&tasks)
	c.JSON(http.StatusOK, tasks)
}

// CreateTask handles POST /api/ping/tasks
func CreateTask(c *gin.Context) {
	var task models.PingTask
	if err := c.ShouldBindJSON(&task); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	dbcore.DB().Create(&task)

	// Broadcast to all agents
	params, _ := json.Marshal(task)
	hub := ws.GetHub()
	hub.BroadcastToAgents(&ws.RPCMessage{
		JSONRPC: "2.0",
		Method:  "master.startPing",
		Params:  params,
	})

	c.JSON(http.StatusOK, task)
}

// GetResults handles GET /api/ping/tasks/:id/results
func GetResults(c *gin.Context) {
	taskID := c.Param("id")

	var results []models.PingResult
	dbcore.DB().Where("task_id = ?", taskID).
		Order("timestamp DESC").
		Limit(100).
		Find(&results)

	c.JSON(http.StatusOK, results)
}

// DeleteTask handles DELETE /api/ping/tasks/:id
func DeleteTask(c *gin.Context) {
	id := c.Param("id")

	// Ask all connected agents to stop this task immediately.
	if taskID, err := strconv.ParseUint(id, 10, 64); err == nil {
		params, _ := json.Marshal(map[string]uint{
			"task_id": uint(taskID),
		})
		ws.GetHub().BroadcastToAgents(&ws.RPCMessage{
			JSONRPC: "2.0",
			Method:  "master.stopPing",
			Params:  params,
		})
	}

	dbcore.DB().Delete(&models.PingTask{}, id)
	dbcore.DB().Where("task_id = ?", id).Delete(&models.PingResult{})
	c.JSON(http.StatusOK, gin.H{"message": "deleted"})
}
