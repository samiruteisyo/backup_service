package main

import (
	"bufio"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

type dbImagePattern struct {
	substring   string
	dbType      DBType
	defaultPort int
}

var dbImagePatterns = []dbImagePattern{
	{"postgres", DBPostgres, 5432},
	{"mysql", DBMySQL, 3306},
	{"mariadb", DBMariaDB, 3306},
	{"mongo", DBMongo, 27017},
	{"mongodb", DBMongo, 27017},
}

type dbEnvMap struct {
	database []string
	user     []string
	password []string
}

var dbEnvMaps = map[DBType]dbEnvMap{
	DBPostgres: {
		database: []string{"POSTGRES_DB", "POSTGRES_DATABASE"},
		user:     []string{"POSTGRES_USER"},
		password: []string{"POSTGRES_PASSWORD"},
	},
	DBMySQL: {
		database: []string{"MYSQL_DATABASE"},
		user:     []string{"MYSQL_USER", "MARIADB_USER"},
		password: []string{"MYSQL_PASSWORD", "MARIADB_PASSWORD"},
	},
	DBMariaDB: {
		database: []string{"MYSQL_DATABASE", "MARIADB_DATABASE"},
		user:     []string{"MYSQL_USER", "MARIADB_USER"},
		password: []string{"MYSQL_PASSWORD", "MARIADB_PASSWORD"},
	},
	DBMongo: {
		database: []string{"MONGO_INITDB_DATABASE"},
		user:     []string{"MONGO_INITDB_ROOT_USERNAME"},
		password: []string{"MONGO_INITDB_ROOT_PASSWORD"},
	},
}

func envMapFromEnvironment(env interface{}, projectEnv map[string]string) map[string]string {
	result := make(map[string]string)
	switch v := env.(type) {
	case map[string]interface{}:
		for k, val := range v {
			strVal, ok := val.(string)
			if !ok {
				continue
			}
			result[k] = resolveEnvValue(strVal, projectEnv)
		}
	case []interface{}:
		for _, item := range v {
			strVal, ok := item.(string)
			if !ok {
				continue
			}
			if idx := strings.IndexByte(strVal, '='); idx != -1 {
				key := strings.TrimSpace(strVal[:idx])
				value := strings.TrimSpace(strVal[idx+1:])
				result[key] = resolveEnvValue(value, projectEnv)
			}
		}
	}
	return result
}

func volumesList(vol interface{}) []interface{} {
	switch v := vol.(type) {
	case []interface{}:
		return v
	case nil:
		return nil
	default:
		return []interface{}{v}
	}
}

var envVarRegex = regexp.MustCompile(`^\$\{(.+?)(?::([?+-])([^}]*))?\}$`)

func parseEnvFile(envPath string) map[string]string {
	data, err := os.ReadFile(envPath)
	if err != nil {
		return nil
	}

	env := make(map[string]string)
	scanner := bufio.NewScanner(strings.NewReader(string(data)))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		eqIdx := strings.Index(line, "=")
		if eqIdx == -1 {
			continue
		}
		key := strings.TrimSpace(line[:eqIdx])
		value := strings.TrimSpace(line[eqIdx+1:])
		if len(value) >= 2 && ((value[0] == '"' && value[len(value)-1] == '"') || (value[0] == '\'' && value[len(value)-1] == '\'')) {
			value = value[1 : len(value)-1]
		}
		env[key] = value
	}
	return env
}

func resolveEnvValue(value string, env map[string]string) string {
	match := envVarRegex.FindStringSubmatch(value)
	if match == nil {
		return value
	}
	key := match[1]
	if v, ok := env[key]; ok {
		return v
	}
	if match[2] == "-" {
		return match[3]
	}
	return ""
}

func parseComposeFile(composePath string) *Project {
	data, err := os.ReadFile(composePath)
	if err != nil {
		return nil
	}

	var compose ComposeFile
	if err := yaml.Unmarshal(data, &compose); err != nil {
		return nil
	}

	projectDir := filepath.Dir(composePath)
	serviceName := filepath.Base(projectDir)
	projectEnv := parseEnvFile(filepath.Join(projectDir, ".env"))

	var database *DatabaseInfo
	bindMountMap := make(map[string]BindMount)
	hasBuild := false

	envMap := make(map[string]interface{})
	for name, svc := range compose.Services {
		db := detectDatabase(name, envMapFromEnvironment(svc.Environment, projectEnv), svc)
		if db != nil {
			database = db
		}

		if svc.Build != nil {
			hasBuild = true
		}

		for _, mount := range extractBindMounts(svc.Volumes, projectDir) {
			bindMountMap[mount.Source] = mount
		}

		if svc.Environment != nil {
			for k, v := range envMapFromEnvironment(svc.Environment, nil) {
				envMap[k] = v
			}
		}
	}

	_ = envMap

	bindMounts := make([]BindMount, 0, len(bindMountMap))
	for _, m := range bindMountMap {
		bindMounts = append(bindMounts, m)
	}

	return &Project{
		Name:        serviceName,
		ComposeFile: filepath.Base(composePath),
		ComposePath: composePath,
		ProjectDir:  projectDir,
		Database:    database,
		BindMounts:  bindMounts,
		HasBuild:    hasBuild,
	}
}

func detectDatabase(serviceName string, envObj map[string]string, svc ComposeService) *DatabaseInfo {
	image := strings.ToLower(svc.Image)

	for _, pattern := range dbImagePatterns {
		if !strings.Contains(image, pattern.substring) {
			continue
		}

		envMap := dbEnvMaps[pattern.dbType]
		creds := DBCredentials{
			Port: pattern.defaultPort,
		}

		for _, key := range envMap.database {
			if v, ok := envObj[key]; ok && v != "" {
				creds.Database = v
				break
			}
		}
		for _, key := range envMap.user {
			if v, ok := envObj[key]; ok && v != "" {
				creds.User = v
				break
			}
		}
		for _, key := range envMap.password {
			if v, ok := envObj[key]; ok && v != "" {
				creds.Password = v
				break
			}
		}

		containerName := svc.ContainerName
		if containerName == "" {
			containerName = serviceName
		}

		return &DatabaseInfo{
			Type:          pattern.dbType,
			ContainerName: containerName,
			ServiceName:   serviceName,
			Credentials:   creds,
		}
	}

	return nil
}

func extractBindMounts(vol interface{}, composeDir string) []BindMount {
	vols := volumesList(vol)
	if vols == nil {
		return nil
	}

	var mounts []BindMount
	for _, vol := range vols {
		var source, target string

		switch v := vol.(type) {
		case string:
			parts := strings.SplitN(v, ":", 2)
			if len(parts) < 2 {
				continue
			}
			source = parts[0]
			target = parts[1]
		case map[string]interface{}:
			volType, _ := v["type"].(string)
			if volType != "bind" {
				continue
			}
			src, _ := v["source"].(string)
			tgt, _ := v["target"].(string)
			source = src
			target = tgt
		default:
			continue
		}

		if source == "" {
			continue
		}

		if source[0] == '.' || source[0] != '/' {
			source = filepath.Join(composeDir, source)
		}

		source = filepath.Clean(source)

		if !strings.HasPrefix(source, composeDir) {
			continue
		}

		if _, err := os.Stat(source); err != nil {
			continue
		}

		mounts = append(mounts, BindMount{
			Source:        source,
			ContainerPath: target,
		})
	}

	return mounts
}
