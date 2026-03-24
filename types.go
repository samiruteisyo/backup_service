package main

import "time"

type DBType string

const (
	DBPostgres DBType = "postgres"
	DBMySQL    DBType = "mysql"
	DBMariaDB  DBType = "mariadb"
	DBMongo    DBType = "mongo"
)

type Config struct {
	ScanPath       string
	BackupPath     string
	Schedule       string
	RetentionDays  int
	RetentionWeeks int
	SkipDirs       []string
	WebPort        int
	AuthUser       string
	AuthPass       string
}

type DatabaseInfo struct {
	Type          DBType
	ContainerName string
	ServiceName   string
	Credentials   DBCredentials
}

type DBCredentials struct {
	Host     string
	Port     int
	Database string
	User     string
	Password string
}

type BindMount struct {
	Source        string
	ContainerPath string
}

type Project struct {
	Name        string
	ComposeFile string
	ComposePath string
	ProjectDir  string
	Database    *DatabaseInfo
	BindMounts  []BindMount
	HasBuild    bool
}

type BackupResult struct {
	ServiceName string
	Type        string
	FilePath    string
	SizeBytes   int64
	Timestamp   time.Time
	Status      string
	Message     string
}

type Deployment struct {
	SHA       string    `json:"sha"`
	Branch    string    `json:"branch"`
	Timestamp time.Time `json:"timestamp"`
	Status    string    `json:"status"`
	Message   string    `json:"message"`
}

type Activity struct {
	Type      string    `json:"type"`
	Timestamp time.Time `json:"timestamp"`
	Message   string    `json:"message"`
	Status    string    `json:"status"`
}

type BackupMeta struct {
	SHA       string `json:"sha"`
	Branch    string `json:"branch"`
	Timestamp string `json:"timestamp"`
}

type RotationResult struct {
	Service string
	Kept    int
	Deleted int
}

type ProjectStatus struct {
	Project     *Project
	Branch      string
	SHA         string
	Ahead       int
	Behind      int
	LastBackup  *time.Time
	LastDeploy  *Deployment
	BackupCount int
	TotalSize   int64
}

type ComposeService struct {
	Image         string      `yaml:"image"`
	ContainerName string      `yaml:"container_name"`
	Build         interface{} `yaml:"build"`
	Environment   interface{} `yaml:"environment"`
	Volumes       interface{} `yaml:"volumes"`
}

type ComposeFile struct {
	Services map[string]ComposeService `yaml:"services"`
}
