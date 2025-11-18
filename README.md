# get_jobs_go 

# model 模型层 没有依赖
# config 配置层 没有依赖
# utils 工具层 没有依赖
# loctors 层 没有依赖

# repository 数据层 依赖 model层 
## boss_repository 数据层 依赖 model层
## ai_repository 数据层 依赖 model层
## cookie_repository 数据层 依赖 model层

# service 服务层 依赖 repository层 model层 
## boss_service 服务层 依赖 repository层 model层 config层
## ai_service 服务层 依赖 repository层 model层 
## cookie_service 服务层 依赖 repository层 model层 

# 入口 worker -> service ->executeDelivery() 
# worker

## boss_worker 服务层 依赖 boss_service层 ai_service层 model层 config层 uitls层
