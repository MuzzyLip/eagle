package app

import (
	"context"
	"errors"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/google/uuid"
	"golang.org/x/sync/errgroup"

	"github.com/go-eagle/eagle/pkg/log"
	"github.com/go-eagle/eagle/pkg/registry"
	"github.com/go-eagle/eagle/pkg/transport"
)

// App global app
type App struct {
	opts     options
	ctx      context.Context
	cancel   func()
	mu       sync.Mutex
	instance *registry.ServiceInstance
}

// New create a app globally
// 创建一个应用程序实例，传入的options是应用程序的配置
func New(opts ...Option) *App {
	o := options{
		ctx: context.Background(),
		// 获取业务层使用的日志实例
		logger: log.GetLogger(),
		// don not catch SIGKILL signal, need to waiting for kill self by other.
		// 获取操作系统信号，用于优雅关闭应用程序
		sigs:            []os.Signal{syscall.SIGTERM, syscall.SIGQUIT, syscall.SIGINT},
		registryTimeout: 10 * time.Second,
	}
	if id, err := uuid.NewUUID(); err == nil {
		o.id = id.String()
	}
	// 为options设置参数
	for _, opt := range opts {
		opt(&o)
	}

	// 创建一个上下文，用于取消应用程序
	ctx, cancel := context.WithCancel(o.ctx)
	return &App{
		opts:   o,
		ctx:    ctx,
		cancel: cancel,
	}
}

// Run start app
// 启动应用程序
// 支持同时起 HTTP gRPC 消息消费者 定时任务Server等
func (a *App) Run() error {
	// build service instance
	instance, err := a.buildInstance()
	if err != nil {
		return err
	}

	// 创建一个错误组，用于管理多个goroutine
	eg, ctx := errgroup.WithContext(a.ctx)

	// start server
	// 创建一个等待组，用于管理多个goroutine
	wg := sync.WaitGroup{}
	// 遍历servers，利用goroutine启动每个server
	for _, srv := range a.opts.servers {
		srv := srv
		// 启动一个协程用于等待Done信号
		eg.Go(func() error {
			// wait for stop signal
			// 这里就会阻塞等待ctx.Done这个停止信号
			<-ctx.Done()
			return srv.Stop(ctx)
		})
		// 添加一个等待组，用于管理多个goroutine
		wg.Add(1)
		// 启动一个协程用于启动server
		eg.Go(func() error {
			// 启动server
			wg.Done()
			return srv.Start(ctx)
		})
	}

	// register service
	// 如果registry不为空，则注册服务
	// 这里Registry和Discovery是一个服务注册与服务发现抽象层
	// 解决的问题是服务实例启动了，需要向注册中心注册，这样其他服务才能发现这个服务实例
	if a.opts.registry != nil {
		c, cancel := context.WithTimeout(a.opts.ctx, a.opts.registryTimeout)
		defer cancel() // 使用defer来延迟执行取消函数，确保在函数退出前取消注册
		// 调用注册中心的Register方法注册服务
		if err := a.opts.registry.Register(c, instance); err != nil {
			return err
		}
		a.mu.Lock()
		a.instance = instance
		a.mu.Unlock()
	}

	// watch signal
	// 创建一个通道，用于接收操作系统信号
	quit := make(chan os.Signal, 1)
	// 监听操作系统信号
	signal.Notify(quit, a.opts.sigs...)
	// 启动一个协程用于监听操作系统信号
	eg.Go(func() error {
		for {
			select {
			// 如果上下文取消，则返回错误
			case <-ctx.Done():
				// 返回上下文取消的错误
				return ctx.Err()
			// 如果收到操作系统信号，则停止应用程序
			case s := <-quit:
				// 记录日志
				a.opts.logger.Infof("receive a quit signal: %s", s.String())
				err := a.Stop()
				if err != nil {
					a.opts.logger.Infof("failed to stop app, err: %s", err.Error())
					return err
				}
			}
		}
	})
	// 等待所有goroutine完成
	// 如果错误不是上下文取消，则返回错误
	// 这里eg.Wait()会阻塞等待所有goroutine完成
	if err := eg.Wait(); err != nil && !errors.Is(err, context.Canceled) {
		return err
	}

	return nil
}

// Stop stops the application gracefully.
func (a *App) Stop() error {
	// deregister instance
	a.mu.Lock()
	instance := a.instance
	a.mu.Unlock()
	if a.opts.registry != nil && instance != nil {
		ctx, cancel := context.WithTimeout(a.opts.ctx, a.opts.registryTimeout)
		defer cancel()
		if err := a.opts.registry.Deregister(ctx, instance); err != nil {
			return err
		}
	}

	// cancel app
	if a.cancel != nil {
		a.cancel()
	}
	return nil
}

func (a *App) buildInstance() (*registry.ServiceInstance, error) {
	// register instance by withEndpoint
	endpoints := make([]string, 0)
	// 当前项目中没有用到endpoints
	for _, e := range a.opts.endpoints {
		endpoints = append(endpoints, e.String())
	}
	// auto register instance
	if len(endpoints) == 0 {
		// 如果endpoints为空，则遍历servers，获取每个server的endpoint
		for _, srv := range a.opts.servers {
			if r, ok := srv.(transport.Endpoint); ok {
				// 得到endpoint地址
				e, err := r.Endpoint()
				if err != nil {
					return nil, err
				}
				endpoints = append(endpoints, e.String())
			}
		}
	}
	return &registry.ServiceInstance{
		ID:        a.opts.id,
		Name:      a.opts.name,
		Version:   a.opts.version,
		Metadata:  a.opts.metadata,
		Endpoints: endpoints,
	}, nil
}
