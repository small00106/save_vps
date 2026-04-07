package models

import (
	"time"

	"gorm.io/gorm"
)

// ==================== 节点相关 ====================

type Node struct {
	UUID      string    `gorm:"primaryKey;size:36" json:"uuid"`
	Token     string    `gorm:"size:64;uniqueIndex" json:"-"`
	Hostname  string    `json:"hostname"`
	IP        string    `json:"ip"`
	Port      int       `gorm:"default:8801" json:"port"`
	Region    string    `json:"region"`
	Tags      string    `gorm:"type:text" json:"tags"` // JSON array
	OS        string    `json:"os"`
	Arch      string    `json:"arch"`
	CPUModel  string    `json:"cpu_model"`
	CPUCores  int       `json:"cpu_cores"`
	DiskTotal int64     `json:"disk_total"`
	DiskUsed  int64     `json:"disk_used"`
	RAMTotal  int64     `json:"ram_total"`
	Status    string    `gorm:"default:online" json:"status"`
	Version   string    `json:"version"`
	RateLimit int64     `gorm:"default:0" json:"rate_limit"`
	LastSeen  time.Time `json:"last_seen"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type NodeMetric struct {
	ID           uint      `gorm:"primaryKey" json:"id"`
	NodeUUID     string    `gorm:"index;size:36" json:"node_uuid"`
	CPUPercent   float64   `json:"cpu_percent"`
	MemPercent   float64   `json:"mem_percent"`
	SwapUsed     int64     `json:"swap_used"`
	SwapTotal    int64     `json:"swap_total"`
	DiskPercent  float64   `json:"disk_percent"`
	Load1        float64   `json:"load1"`
	Load5        float64   `json:"load5"`
	Load15       float64   `json:"load15"`
	NetInSpeed   int64     `json:"net_in_speed"`
	NetOutSpeed  int64     `json:"net_out_speed"`
	NetInTotal   int64     `json:"net_in_total"`
	NetOutTotal  int64     `json:"net_out_total"`
	TCPConns     int       `json:"tcp_conns"`
	UDPConns     int       `json:"udp_conns"`
	ProcessCount int       `json:"process_count"`
	Uptime       int64     `json:"uptime"`
	Timestamp    time.Time `gorm:"index" json:"timestamp"`
}

type NodeMetricCompact struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	NodeUUID    string    `gorm:"index;size:36" json:"node_uuid"`
	CPUPercent  float64   `json:"cpu_percent"`
	MemPercent  float64   `json:"mem_percent"`
	DiskPercent float64   `json:"disk_percent"`
	NetInSpeed  int64     `json:"net_in_speed"`
	NetOutSpeed int64     `json:"net_out_speed"`
	BucketTime  time.Time `gorm:"index" json:"bucket_time"`
}

// ==================== 存储相关 ====================

type File struct {
	FileID    string         `gorm:"primaryKey;size:36" json:"file_id"`
	Name      string         `json:"name"`
	Path      string         `gorm:"index" json:"path"`
	Size      int64          `json:"size"`
	MimeType  string         `json:"mime_type"`
	Checksum  string         `json:"checksum"`
	IsDir     bool           `gorm:"default:false" json:"is_dir"`
	Status    string         `gorm:"default:uploading" json:"status"`
	CreatedAt time.Time      `json:"created_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"deleted_at"`
}

type FileReplica struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	FileID     string    `gorm:"index;size:36" json:"file_id"`
	NodeUUID   string    `gorm:"index;size:36" json:"node_uuid"`
	Status     string    `json:"status"`
	StorePath  string    `json:"store_path"`
	VerifiedAt time.Time `json:"verified_at"`
}

// ==================== 认证 ====================

type User struct {
	ID           uint      `gorm:"primaryKey" json:"id"`
	Username     string    `gorm:"uniqueIndex" json:"username"`
	PasswordHash string    `json:"-"`
	CreatedAt    time.Time `json:"created_at"`
}

type Session struct {
	Token     string    `gorm:"primaryKey;size:64" json:"token"`
	UserID    uint      `json:"user_id"`
	ExpiresAt time.Time `json:"expires_at"`
}

// ==================== 告警 ====================

type AlertRule struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	Name        string    `json:"name"`
	NodeUUID    string    `gorm:"index;size:36" json:"node_uuid"`
	Metric      string    `json:"metric"`
	Operator    string    `json:"operator"`
	Threshold   float64   `json:"threshold"`
	Duration    int       `json:"duration"`
	ChannelID   uint      `json:"channel_id"`
	Enabled     bool      `gorm:"default:true" json:"enabled"`
	LastFiredAt time.Time `json:"last_fired_at"`
	CreatedAt   time.Time `json:"created_at"`
}

type AlertChannel struct {
	ID     uint   `gorm:"primaryKey" json:"id"`
	Name   string `json:"name"`
	Type   string `json:"type"`
	Config string `gorm:"type:text" json:"config"`
}

// ==================== Ping 探测 ====================

type PingTask struct {
	ID       uint   `gorm:"primaryKey" json:"id"`
	Name     string `json:"name"`
	Type     string `json:"type"`
	Target   string `json:"target"`
	Interval int    `json:"interval"`
	Enabled  bool   `gorm:"default:true" json:"enabled"`
}

type PingResult struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	TaskID    uint      `gorm:"index" json:"task_id"`
	NodeUUID  string    `gorm:"index;size:36" json:"node_uuid"`
	Latency   float64   `json:"latency"`
	Success   bool      `json:"success"`
	Timestamp time.Time `gorm:"index" json:"timestamp"`
}

// ==================== 远程操作 ====================

type CommandTask struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	NodeUUID  string    `gorm:"index;size:36" json:"node_uuid"`
	Command   string    `gorm:"type:text" json:"command"`
	Output    string    `gorm:"type:text" json:"output"`
	ExitCode  int       `json:"exit_code"`
	Status    string    `gorm:"default:pending" json:"status"`
	CreatedAt time.Time `json:"created_at"`
}

type AuditLog struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	Action     string    `gorm:"index;size:64" json:"action"`
	Actor      string    `gorm:"size:64" json:"actor"`
	Status     string    `gorm:"index;size:16" json:"status"`
	TargetType string    `gorm:"size:64" json:"target_type"`
	TargetID   string    `gorm:"size:64" json:"target_id"`
	NodeUUID   string    `gorm:"index;size:36" json:"node_uuid"`
	Detail     string    `gorm:"type:text" json:"detail"`
	IP         string    `json:"ip"`
	CreatedAt  time.Time `json:"created_at"`
}

// ==================== 系统设置 ====================

type Setting struct {
	Key   string `gorm:"primaryKey;size:64" json:"key"`
	Value string `gorm:"type:text" json:"value"`
}
