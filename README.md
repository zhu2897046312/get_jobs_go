# 求职信息采集系统 (Get Jobs Go)

基于 Go 和 Playwright 的自动化求职信息采集系统，参考 [get_jobs](https://github.com/loks666/get_jobs) 项目，专门用于 Boss 直聘平台的数据采集和自动化投递。

## 🚀 功能特性

- 🤖 **智能自动化** - 基于 Playwright 的浏览器自动化操作
- 📊 **数据采集** - 自动采集 Boss 直聘职位信息
- 💼 **简历投递** - 支持自动投递简历功能
- 🗄️ **数据存储** - 使用 MySQL 数据库存储采集数据
- ⚙️ **配置管理** - 灵活的配置系统支持多种采集策略
- 🔒 **Cookie 管理** - 智能管理登录状态和会话
- 📈 **进度监控** - 实时任务进度跟踪和状态反馈
- 🎯 **精准匹配** - AI 辅助职位匹配和筛选

## 📋 系统要求

- Go 1.19 或更高版本
- MySQL 5.7 或更高版本
- 支持的操作系统：Windows, macOS, Linux

## 🛠️ 快速开始

### 1. 克隆项目

```bash
git clone <项目地址>
cd get_jobs_go
```

### 2. 安装依赖

```bash
go mod tidy
```

### 3. 数据库配置

创建 MySQL 数据库并修改配置：

```bash
# 创建数据库
mysql -u root -p -e "CREATE DATABASE jobs CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;"

# 配置信息在 main.go 中修改：
# dsn := "root:你的密码@tcp(localhost:3306)/jobs?charset=utf8mb4&parseTime=True&loc=Local"
```

### 4. 运行系统

```bash
go run main.go
```

## 🏗️ 项目结构

```
get_jobs_go/
├── config/           # 配置管理
├── model/            # 数据模型
├── repository/       # 数据访问层
├── service/          # 业务逻辑层
├── worker/           # 工作器模块
│   ├── boss/         # Boss 直聘采集器
│   └── playwright_manager/  # 浏览器管理
├── main.go           # 程序入口
└── README.md         # 项目说明
```

## ⚙️ 配置说明

### 数据库配置

在 `main.go` 中修改数据库连接字符串：

```go
dsn := "用户名:密码@tcp(主机:端口)/数据库名?charset=utf8mb4&parseTime=True&loc=Local"
```

### 应用配置

通过环境变量或配置文件设置：

```bash
# 可选：设置配置文件路径
export CONFIG_PATH="./config.yaml"
```

## 🔧 核心模块

### Boss 直聘采集器 (`worker/boss`)

- 职位信息采集
- 自动简历投递
- 聊天消息处理
- 图片简历发送
- 智能沟通回复

### 浏览器管理器 (`worker/playwright_manager`)

- 浏览器实例管理
- 页面生命周期控制
- Cookie 状态维护
- 异常恢复处理
- 反检测机制

### 数据服务层 (`service`)

- 配置管理服务
- Cookie 管理服务
- AI 辅助服务
- Boss 平台服务
- 黑名单管理

## 📊 数据库表结构

系统自动创建以下表：
- `ai_entities` - AI 配置信息
- `blacklist_entities` - 黑名单管理
- `boss_config_entities` - Boss 平台配置
- `boss_industry_entities` - 行业分类
- `boss_job_data_entities` - 职位数据
- `boss_option_entities` - 平台选项
- `config_entities` - 系统配置
- `cookie_entities` - Cookie 存储
- `jobs` - 职位信息

## 🎯 使用方法

### 启动系统

```bash
go run main.go
```

系统将自动：
1. 初始化数据库连接
2. 启动 Playwright 浏览器实例
3. 开始 Boss 直聘数据采集任务
4. 监听系统退出信号，实现优雅关闭

### 停止系统

使用 `Ctrl + C` 发送中断信号，系统将：
1. 停止所有采集任务
2. 关闭浏览器实例
3. 释放数据库连接
4. 安全退出程序

## 📝 与原项目对比

### 改进特性
- ✅ **Go 语言重构** - 更好的性能和并发处理
- ✅ **模块化设计** - 更清晰的代码结构
- ✅ **错误处理** - 完善的错误处理和恢复机制
- ✅ **配置管理** - 灵活的配置系统
- ✅ **数据库优化** - 使用 GORM ORM 框架

### 保留功能
- ✅ Boss 直聘数据采集
- ✅ 自动简历投递
- ✅ 图片简历发送
- ✅ 智能沟通回复
- ✅ 黑名单管理

## ⚠️ 注意事项

1. **合规使用**：请遵守 Boss 直聘平台的使用条款，合理设置采集频率
2. **账号安全**：妥善保管登录凭证，避免账号被封禁
3. **网络环境**：确保稳定的网络连接，避免采集中断
4. **资源占用**：系统会启动浏览器实例，请确保有足够的内存资源
5. **法律风险**：仅用于学习和研究目的，请遵守相关法律法规

## 🐛 故障排除

### 常见问题

1. **数据库连接失败**
   - 检查 MySQL 服务是否启动
   - 验证连接字符串的用户名和密码

2. **Playwright 初始化失败**
   - 运行 `go mod tidy` 确保依赖完整
   - 检查系统是否支持浏览器自动化

3. **采集任务异常**
   - 检查网络连接
   - 验证 Boss 直聘账号状态
   - 查看日志输出定位具体问题

### 日志查看

系统会输出详细的运行日志，包括：
- 服务初始化状态
- 数据采集进度
- 错误和警告信息
- 性能统计指标

## 🔮 开发计划

- [ ] 支持更多招聘平台（智联、51job 等）
- [ ] 添加 Web 管理界面
- [ ] 实现分布式采集
- [ ] 添加数据分析和报表功能
- [ ] 支持代理和轮换账号

## 🤝 贡献指南

欢迎提交 Issue 和 Pull Request 来改进这个项目。

1. Fork 本仓库
2. 创建功能分支 (`git checkout -b feature/AmazingFeature`)
3. 提交更改 (`git commit -m 'Add some AmazingFeature'`)
4. 推送到分支 (`git push origin feature/AmazingFeature`)
5. 开启 Pull Request

## 📄 许可证

本项目基于 MIT 许可证开源 - 查看 [LICENSE](LICENSE) 文件了解详情。

## 🙏 致谢

感谢原项目 [get_jobs](https://github.com/loks666/get_jobs) 的启发和参考。

---

**免责声明**: 本项目仅用于技术学习和研究目的，请遵守相关网站的使用条款和法律法规。使用者应对自己的行为负责，作者不承担任何法律责任。