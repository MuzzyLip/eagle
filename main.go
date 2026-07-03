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
	_ "go.uber.org/automaxprocs"

	"github.com/go-eagle/eagle/internal/model"
	"github.com/go-eagle/eagle/internal/repository"
	"github.com/go-eagle/eagle/internal/server"
	"github.com/go-eagle/eagle/internal/service"
	eagle "github.com/go-eagle/eagle/pkg/app"
	"github.com/go-eagle/eagle/pkg/config"
	logger "github.com/go-eagle/eagle/pkg/log"
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
	logger.Init()
	// init db
	model.Init()
	// init redis
	// nolint: errcheck
	// redis.Init()

	// init service
	db, _ := model.GetDB()
	service.Svc = service.New(repository.New(db))

	gin.SetMode(cfg.Mode)

	// init pprof server
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

	if err := app.Run(); err != nil {
		panic(err)
	}
}
