package config

import (
	"errors"
	"log"
	"os"
	"path/filepath"
	"sync"

	"github.com/fsnotify/fsnotify"
	"github.com/spf13/viper"
)

// 这里定义FileType AI建议使用常量来定义文件类型，因为文件类型是固定的，不会改变
var (
	// conf conf var
	conf *Config

	// conf file type
	FileTypeYaml = "yaml"
	FileTypeJson = "json"
	FileTypeToml = "toml"
)

// Config conf struct.
type Config struct {
	env        string
	configDir  string
	configType string // file type, eg: yaml, json, toml, default is yaml
	val        map[string]*viper.Viper
	// 通过Mutex互斥锁保证并发安全，主要保护val map并发读写竟态导致 concurrent map read and map write 这类问题
	mu sync.Mutex
}

// New create a config instance.
func New(cfgDir string, opts ...Option) *Config {
	// must set config dir
	if cfgDir == "" {
		panic("config dir is not set")
	}
	c := Config{
		configDir:  cfgDir,
		configType: FileTypeYaml,
		val:        make(map[string]*viper.Viper),
	}
	// opts目前是传入的Option函数，用于设置config对象的属性(目前设置的是env)
	for _, opt := range opts {
		opt(&c)
	}

	conf = &c

	return &c
}

// Load alias for config func.
func Load(filename string, val interface{}) error { return conf.Load(filename, val) }

// Load scan data to struct.
func (c *Config) Load(filename string, val interface{}) error {
	v, err := c.LoadWithType(filename, c.configType)
	if err != nil {
		return err
	}
	err = v.Unmarshal(&val)
	if err != nil {
		return err
	}
	return nil
}

// LoadYaml alias for config func.
func LoadYaml(filename string, val interface{}) error { return conf.LoadYaml(filename, val) }

// LoadYaml scan data to struct.
func (c *Config) LoadYaml(filename string, val interface{}) error {
	v, err := c.LoadWithType(filename, FileTypeYaml)
	if err != nil {
		return err
	}
	err = v.Unmarshal(&val)
	if err != nil {
		return err
	}
	return nil
}

// LoadJson alias for config func.
func LoadJson(filename string, val interface{}) error { return conf.LoadJson(filename, val) }

// LoadJson scan data to struct.
func (c *Config) LoadJson(filename string, val interface{}) error {
	v, err := c.LoadWithType(filename, FileTypeJson)
	if err != nil {
		return err
	}
	err = v.Unmarshal(&val)
	if err != nil {
		return err
	}
	return nil
}

// LoadToml alias for config func.
func LoadToml(filename string, val interface{}) error { return conf.LoadToml(filename, val) }

// LoadToml scan data to struct.
func (c *Config) LoadToml(filename string, val interface{}) error {
	v, err := c.LoadWithType(filename, FileTypeToml)
	if err != nil {
		return err
	}
	err = v.Unmarshal(&val)
	if err != nil {
		return err
	}
	return nil
}

// LoadWithType load conf by file type.
func LoadWithType(filename string, cfgType string) (*viper.Viper, error) {
	return conf.LoadWithType(filename, cfgType)
}

// LoadWithType load conf by file type.
func (c *Config) LoadWithType(filename string, cfgType string) (v *viper.Viper, err error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	// 先从val map中尝试获取已加载的配置
	v, ok := c.val[filename]
	// 如果存在，则直接返回已加载的配置
	if ok {
		return v, nil
	}

	// 如果不存在，则调用load方法加载配置
	v, err = c.load(filename, cfgType)
	if err != nil {
		return nil, err
	}
	// 将加载的配置保存到val map中，以便下次直接从val map中获取
	c.val[filename] = v
	return v, nil
}

// Load load file.
func (c *Config) load(filename string, cfgType string) (*viper.Viper, error) {
	// application parameters take precedence over environment variables
	env := GetEnvString("APP_ENV", "")
	// 从configDir目录下，根据环境变量或传入的env参数，拼接出配置文件路径
	// 这里env就是config/下的目录
	path := filepath.Join(c.configDir, env)
	// 如果传入了env参数，则优先使用传入的env参数
	if c.env != "" {
		path = filepath.Join(c.configDir, c.env)
	}

	// 通过viper加载配置文件
	v := viper.New()
	// 添加配置文件路径
	v.AddConfigPath(path)
	// 设置配置文件名称
	v.SetConfigName(filename)
	// 设置配置文件类型
	v.SetConfigType(c.configType)
	// 如果传入了cfgType参数，则优先使用传入的cfgType参数
	if cfgType != "" {
		v.SetConfigType(cfgType)
	}

	// 读取配置文件
	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			return nil, errors.New("config file not found")
		}
		return nil, err
	}

	// 监听配置文件变化
	v.WatchConfig()
	// 当配置文件变化时，打印日志
	v.OnConfigChange(func(e fsnotify.Event) {
		log.Printf("Config file changed: %s", e.Name)
		// 可以显示重新加载配置，但这里暂时没有实现
	})

	return v, nil
}

// GetEnvString get value from env.
func GetEnvString(key string, defaultValue string) string {
	val, ok := os.LookupEnv(key)
	if !ok {
		return defaultValue
	}
	return val
}
