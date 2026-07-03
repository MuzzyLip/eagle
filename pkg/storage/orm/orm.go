package orm

import (
	"database/sql"
	"fmt"
	"sync"
	"time"

	"gorm.io/driver/clickhouse"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/plugin/opentelemetry/tracing"

	"github.com/go-eagle/eagle/pkg/config"
	"github.com/go-eagle/eagle/pkg/log"
)

const (
	// DriverMySQL mysql driver
	DriverMySQL = "mysql"
	// DriverPostgres postgresSQL driver
	DriverPostgres = "postgres"
	// DriverClickhouse
	DriverClickhouse = "clickhouse"

	// DefaultDatabase default db name
	DefaultDatabase = "default"
)

var (
	// DBMap store database instance
	// 包级全局变量，用于存储数据库实例，key是数据库名称，value是数据库实例
	DBMap = make(map[string]*gorm.DB)
	// DBLock database locker
	DBLock sync.Mutex
	// Logger log writer
	Logger log.Logger
)

// Config database config
type Config struct {
	Driver                    string
	Name                      string
	Addr                      string
	UserName                  string
	Password                  string
	MaxIdleConn               int
	MaxOpenConn               int
	Timeout                   string // connect timeout
	ReadTimeout               string
	WriteTimeout              string
	ConnMaxLifeTime           time.Duration
	SlowThreshold             time.Duration // 慢查询时长，默认200ms
	LogLevel                  string
	Colorful                  bool
	IgnoreRecordNotFoundError bool
	EnableTrace               bool
}

// New create a or multi database client
func New(names ...string) error {
	if len(names) == 0 {
		return fmt.Errorf("no set databasename")
	}

	// 这里也是一样的，通过map[string]*gorm.DB来存储数据库实例，key是数据库名称，value是数据库实例
	clientManager := NewManager()
	for _, name := range names {
		// 创建或挂载数据库实例到instances map中
		_, err := clientManager.GetInstance(name)
		if err != nil {
			return fmt.Errorf("init database name: %+v, err: %+v", name, err)
		}
	}

	return nil
}

// Manager define a manager
type Manager struct {
	instances map[string]*gorm.DB
	// 通过RWMutex互斥锁保证并发安全，主要保护instances map并发读写竟态导致 concurrent map read and map write 这类问题
	*sync.RWMutex
}

// NewManager create a database manager
func NewManager() *Manager {
	return &Manager{
		instances: make(map[string]*gorm.DB),
		RWMutex:   &sync.RWMutex{},
	}
}

// GetDB get a database
func GetDB(name string) (*gorm.DB, error) {
	DBLock.Lock()
	defer DBLock.Unlock()

	db, ok := DBMap[name]
	if !ok {
		db, err := NewManager().GetInstance(name)
		if err != nil {
			return nil, err
		}
		return db, nil
	}

	return db, nil
}

// GetInstance return a database client
func (m *Manager) GetInstance(name string) (*gorm.DB, error) {
	// get client from map
	m.RLock()
	// 先从instances map中尝试获取已加载的数据库实例
	if ins, ok := m.instances[name]; ok {
		m.RUnlock()
		return ins, nil
	}
	m.RUnlock()

	// 如果不存在，则调用LoadConf方法加载数据库配置
	c, err := LoadConf(name)
	if err != nil {
		return nil, fmt.Errorf("load database conf err: %+v", err)
	}

	// create a database client
	m.Lock()
	defer m.Unlock()

	instance := NewInstance(c)
	// 挂载到instances map中，以便下次直接从instances map中获取
	m.instances[name] = instance
	DBMap[name] = instance

	return instance, nil
}

// NewInstance connect to database and create a db instance
func NewInstance(c *Config) (db *gorm.DB) {
	var (
		err   error
		sqlDB *sql.DB
	)
	// 通过数据库类型，选择不同的数据库驱动，得到数据库的dsn
	dsn := getDSN(c)
	// 根据数据库驱动，选择不同的数据库驱动，创建数据库实例
	switch c.Driver {
	case DriverMySQL:
		db, err = gorm.Open(mysql.Open(dsn), gormConfig(c))
	case DriverPostgres:
		db, err = gorm.Open(postgres.Open(dsn), gormConfig(c))
	case DriverClickhouse:
		db, err = gorm.Open(clickhouse.Open(dsn), gormConfig(c))
	default:
		db, err = gorm.Open(mysql.Open(dsn), gormConfig(c))
	}
	if err != nil {
		log.Fatalf("open db failed. driver: %s, database name: %s, err: %+v", c.Driver, c.Name, err)
	}

	sqlDB, err = db.DB()
	if err != nil {
		log.Fatalf("database connection failed. database name: %s, err: %+v", c.Name, err)
	}
	// set for db connection
	// 用于设置最大打开的连接数，默认值为0表示不限制.设置最大的连接数，可以避免并发太高导致连接mysql出现too many connections的错误。
	sqlDB.SetMaxOpenConns(c.MaxOpenConn)
	// 用于设置闲置的连接数.设置闲置的连接数则当开启的一个连接使用完成后可以放在池里等候下一次使用。
	sqlDB.SetMaxIdleConns(c.MaxIdleConn)
	sqlDB.SetConnMaxLifetime(c.ConnMaxLifeTime)

	db.Set("gorm:table_options", "CHARSET=utf8mb4")

	// set trace
	if c.EnableTrace {
		err = db.Use(tracing.NewPlugin())
		if err != nil {
			log.Fatalf("using gorm opentelemetry, err: %+v", err)
		}
	}

	return db
}

// LoadConf load database config
func LoadConf(name string) (ret *Config, err error) {
	// 通过 /pkg/config/config.go 中的 LoadWithType 方法指定加载 /config/环境/database.yaml 数据库配置文件
	v, err := config.LoadWithType("database", "yaml")
	if err != nil {
		return nil, err
	}

	var c Config
	err = v.UnmarshalKey(name, &c)
	if err != nil {
		return nil, err
	}

	return &c, nil
}

// getDSN return dsn string
func getDSN(c *Config) string {
	// default mysql
	dsn := fmt.Sprintf("%s:%s@tcp(%s)/%s?charset=utf8mb4&parseTime=true&loc=Local&timeout=%s&readTimeout=%s&writeTimeout=%s",
		c.UserName,
		c.Password,
		c.Addr,
		c.Name,
		c.Timeout,
		c.ReadTimeout,
		c.WriteTimeout,
	)

	if c.Driver == DriverPostgres {
		dsn = fmt.Sprintf("postgres://%s:%s@%s/%s?sslmode=disable&connect_timeout=%s&statement_timeout=%s",
			c.UserName,
			c.Password,
			c.Addr,
			c.Name,
			c.Timeout,
			c.ReadTimeout,
		)
	}

	if c.Driver == DriverClickhouse {
		dsn = fmt.Sprintf("clickhouse://%s:%s@%s/%s?dial_timeout=%s&read_timeout=%s",
			c.UserName,
			c.Password,
			c.Addr,
			c.Name,
			c.Timeout,
			c.ReadTimeout,
		)
	}

	return dsn
}

// gormConfig 根据配置决定是否开启日志
func gormConfig(c *Config) *gorm.Config {
	gormCfg := &gorm.Config{
		DisableForeignKeyConstraintWhenMigrating: true, // 禁止外键约束, 生产环境不建议使用外键约束
	}

	logger := log.GetLogger()
	// 如果需要自定义日志文件名可以传入logger
	if Logger != nil {
		logger = Logger
	}

	gormCfg.Logger = NewGormLogger(logger, c)

	return gormCfg
}
