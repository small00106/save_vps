package files

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/cloudnest/cloudnest/internal/database/dbcore"
	"github.com/cloudnest/cloudnest/internal/database/models"
	"github.com/cloudnest/cloudnest/internal/transfer"
	"github.com/gin-gonic/gin"
)

var agentProxyHTTPClient = &http.Client{Timeout: 60 * time.Second}

func writeProxyResponse(c *gin.Context, resp *http.Response) {
	for k, vs := range resp.Header {
		for _, v := range vs {
			c.Writer.Header().Add(k, v)
		}
	}
	c.Status(resp.StatusCode)
	_, _ = io.Copy(c.Writer, resp.Body)
}

// resolveNode looks up an online node by UUID.
func resolveNode(uuid string) (*models.Node, error) {
	var node models.Node
	if err := dbcore.DB().Where("uuid = ? AND status = ?", uuid, "online").First(&node).Error; err != nil {
		return nil, err
	}
	return &node, nil
}

func signedUploadResource(fileID, path, name, overwrite string) string {
	cleanPath := strings.TrimSpace(path)
	cleanName := strings.TrimSpace(name)
	if cleanName == "" && cleanPath == "" && strings.TrimSpace(overwrite) == "" {
		return fileID
	}
	if cleanPath == "" {
		cleanPath = "/"
	}
	return fmt.Sprintf("%s|%s|%s|%t", fileID, cleanPath, cleanName, strings.EqualFold(strings.TrimSpace(overwrite), "true"))
}

func signedMoveResource(fromPath, toPath string) string {
	return fmt.Sprintf("%s|%s", strings.TrimSpace(fromPath), strings.TrimSpace(toPath))
}

// ProxyUpload handles PUT /api/proxy/upload/:file_id?node=<uuid>
// Streams the browser's request body to the Agent via a signed URL.
func ProxyUpload(c *gin.Context) {
	fileID := c.Param("file_id")
	nodeUUID := c.Query("node")
	if nodeUUID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "node query param required"})
		return
	}

	node, err := resolveNode(nodeUUID)
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "node not available"})
		return
	}

	// Build signed URL for the Agent
	agentBase := fmt.Sprintf("http://%s:%d/api/files/%s", node.IP, node.Port, fileID)
	agentURL := transfer.GenerateSignedURL(
		agentBase,
		signedUploadResource(fileID, c.Query("path"), c.Query("name"), c.Query("overwrite")),
		http.MethodPut,
		5*time.Minute,
	)
	if path := c.Query("path"); path != "" {
		agentURL += "&path=" + url.QueryEscape(path)
	}
	if name := c.Query("name"); name != "" {
		agentURL += "&name=" + url.QueryEscape(name)
	}
	if overwrite := c.Query("overwrite"); overwrite != "" {
		agentURL += "&overwrite=" + url.QueryEscape(overwrite)
	}

	// Create the upstream request, streaming the body directly (no buffering)
	req, err := http.NewRequestWithContext(c.Request.Context(), http.MethodPut, agentURL, c.Request.Body)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create proxy request"})
		return
	}
	req.ContentLength = c.Request.ContentLength
	if ct := c.GetHeader("Content-Type"); ct != "" {
		req.Header.Set("Content-Type", ct)
	}

	resp, err := agentProxyHTTPClient.Do(req)
	if err != nil {
		log.Printf("[Proxy] Upload to agent %s failed: %v", nodeUUID, err)
		c.JSON(http.StatusBadGateway, gin.H{"error": "failed to reach agent"})
		return
	}
	defer resp.Body.Close()

	// Forward the Agent's response back to the browser.
	// Headers must be copied before status is written, otherwise attachment
	// headers may be dropped and the browser can open files inline.
	writeProxyResponse(c, resp)
}

// ProxyDownload handles GET /api/proxy/download/:file_id?node=<uuid>
// Streams the file from Agent back to the browser.
func ProxyDownload(c *gin.Context) {
	fileID := c.Param("file_id")
	nodeUUID := c.Query("node")
	if nodeUUID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "node query param required"})
		return
	}

	node, err := resolveNode(nodeUUID)
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "node not available"})
		return
	}

	agentBase := fmt.Sprintf("http://%s:%d/api/files/%s", node.IP, node.Port, fileID)
	agentURL := transfer.GenerateSignedURL(agentBase, fileID, http.MethodGet, 5*time.Minute)
	if filename := c.Query("filename"); filename != "" {
		agentURL += "&filename=" + url.QueryEscape(filename)
	}

	req, err := http.NewRequestWithContext(c.Request.Context(), http.MethodGet, agentURL, nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create proxy request"})
		return
	}

	resp, err := agentProxyHTTPClient.Do(req)
	if err != nil {
		log.Printf("[Proxy] Download from agent %s failed: %v", nodeUUID, err)
		c.JSON(http.StatusBadGateway, gin.H{"error": "failed to reach agent"})
		return
	}
	defer resp.Body.Close()

	writeProxyResponse(c, resp)
}

// ProxyBrowse handles GET /api/proxy/browse?node=<uuid>&path=<path>
// Streams a file from the Agent's real filesystem (by path) back to the browser.
func ProxyBrowse(c *gin.Context) {
	nodeUUID := c.Query("node")
	filePath := c.Query("path")
	if nodeUUID == "" || filePath == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "node and path query params required"})
		return
	}

	node, err := resolveNode(nodeUUID)
	if err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "node not available"})
		return
	}

	agentBase := fmt.Sprintf("http://%s:%d/api/browse", node.IP, node.Port)
	agentURL := transfer.GenerateSignedURL(agentBase, filePath, http.MethodGet, 5*time.Minute)
	agentURL += "&path=" + url.QueryEscape(filePath)
	if filename := c.Query("filename"); filename != "" {
		agentURL += "&filename=" + url.QueryEscape(filename)
	}
	if archive := c.Query("archive"); archive != "" {
		agentURL += "&archive=" + url.QueryEscape(archive)
	}

	req, err := http.NewRequestWithContext(c.Request.Context(), http.MethodGet, agentURL, nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create proxy request"})
		return
	}

	resp, err := agentProxyHTTPClient.Do(req)
	if err != nil {
		log.Printf("[Proxy] Browse download from agent %s failed: %v", nodeUUID, err)
		c.JSON(http.StatusBadGateway, gin.H{"error": "failed to reach agent"})
		return
	}
	defer resp.Body.Close()

	writeProxyResponse(c, resp)
}
