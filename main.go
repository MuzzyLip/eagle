/**
 *
 *    ____          __
 *   / __/__ ____ _/ /__
 *  / _// _ `/ _ `/ / -_)
 * /___/\_,_/\_, /_/\__/
 *         /___/
 *
 *
 * generate by http://patorjk.com/software/taag/#p=display&f=Small%20Slant&t=Eagle
 */
package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/spf13/pflag"

	// 这个库是用来自动设置GOMAXPROCS的，根据当前机器的CPU核心数来设置GOMAXPROCS
	// 这样可以让Go程序充分利用多核CPU，提高性能
	_ "go.uber.org/automaxprocs"

	"github.com/go-eagle/eagle/internal/model"
	"github.com/go-eagle/eagle/internal/repository"
	"github.com/go-eagle/eagle/internal/server"
	"github.com/go-eagle/eagle/internal/service"
	eagle "github.com/go-eagle/eagle/pkg/app"
	"github.com/go-eagle/eagle/pkg/config"
	logger "github.com/go-eagle/eagle/pkg/log"
	"github.com/go-eagle/eagle/pkg/redis"
	v "github.com/go-eagle/eagle/pkg/version"
)

// pflag 是用来解析命令行参数的，是Go标准库flag的增强版
// 这里就是为了提供命令行参数来配置项目，比如配置文件路径、环境变量、版本信息等
var (
	cfgDir  = pflag.StringP("config dir", "c", "config", "config path.")
	env     = pflag.StringP("env name", "e", "", "env var name.")
	version = pflag.BoolP("version", "v", false, "show version info.")
)

// @title eagle docs api
// @version 1.0
// @description eagle demo

// @host localhost:8080
// @BasePath /v1
func main() {
	// 解析pflag参数
	pflag.Parse()
	if *version {
		// 从本地pkg/version/version.go中获取版本信息
		ver := v.Get()
		// 将版本信息编码成JSON，并带缩进格式输出
		marshaled, err := json.MarshalIndent(&ver, "", "  ")
		if err != nil {
			fmt.Printf("%v\n", err)
			os.Exit(1)
		}

		// 将版本信息打印到控制台
		fmt.Println(string(marshaled))
		return
	}

	// init config
	// 创建一个config对象，并设置环境变量
	c := config.New(*cfgDir, config.WithEnv(*env))
	var cfg eagle.Config
	// 加载app配置文件
	if err := c.Load("app", &cfg); err != nil {
		panic(err)
	}
	// set global
	// 将配置文件加载到全局变量中 /pkg/app/config.go
	eagle.Conf = &cfg

	// -------------- init resource -------------
	// 初始化日志
	logger.Init()
	// init db
	// 初始化数据库
	model.Init()
	// init redis
	// nolint: errcheck
	// 之前这里注释了Redis，但是AI说其实有不少代码依赖了全局的redis.RedisClient，所以这里还是初始化
	redis.Init()

	// init service
	// 获取default database实例
	db, _ := model.GetDB()
	// 绑定全局Svc实例为default database实例的Repository实例
	service.Svc = service.New(repository.New(db))

	// 设置gin模式 为debug、release、test中的一个
	gin.SetMode(cfg.Mode)

	// init pprof server
	// 创建一个goroutine，用于启动pprof服务器
	// pprof是Go性能剖析工具，运行在独立的端口上，方便我们进行性能分析
	go func() {
		fmt.Printf("Listening and serving PProf HTTP on %s\n", cfg.PprofPort)
		if err := http.ListenAndServe(cfg.PprofPort, http.DefaultServeMux); err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen ListenAndServe for PProf, err: %s", err.Error())
		}
	}()

	// start app
	app := eagle.New(
		eagle.WithName(cfg.Name),
		eagle.WithVersion(cfg.Version),
		eagle.WithLogger(logger.GetLogger()),
		eagle.WithServer(
			// init http server
			server.NewHTTPServer(&cfg.HTTP),
		),
	)

	// 启动应用程序
	if err := app.Run(); err != nil {
		panic(err)
	}
}
