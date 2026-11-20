package main

import (
	"FastGo/internal/bootstrap"
	"flag"
	"log"
)

func main() {
	// 1. 定义命令行标志：解析启动参数，决定是否执行数据库迁移
	// 默认值为 false，即正常启动服务
	migrate := flag.Bool("migrate", false, "执行数据库迁移")
	flag.Parse() // 解析 os.Args

	// 2. 初始化应用核心组件（依赖注入/装配过程）
	// Setup 负责加载配置、连接 DB/Redis、初始化日志和路由
	app, err := bootstrap.Setup()
	if err != nil {
		// 如果基础组件初始化失败，直接终止进程
		log.Fatalf("应用初始化失败: %v", err)
	}

	// 3. 执行数据库迁移逻辑
	// 如果命令行传入了 -migrate=true，则执行建表/迁移逻辑
	if *migrate {
		bootstrap.Migrate()
		return // 迁移完成后直接退出，不启动 HTTP 服务
	}

	// 4. 正常启动应用（阻塞运行）
	// 启动 HTTP Server 并监听系统信号
	if err := app.Run(); err != nil {
		log.Fatalf("应用运行失败: %v", err)
	}
}
