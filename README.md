# Fuck-GPU

在GPU卡上按需运行多个实例，让显存尽可能用满，

## 功能特性

1. 获取GPU余量
2. 获取实例需要的GPU数量
3. 确定最大实例数量
4. 创建实例
5. 应用副本管理
6. 自动资源调度
7. 健康检查和重启机制

## 使用说明

### 启动守护进程

```bash
# 启动守护进程
./fuck-gpu daemon

# 或者指定配置文件
./fuck-gpu daemon -c config.yaml
```

### 查看GPU信息

```bash
# 查看GPU内存信息
./fuck-gpu gpu-collect
```

## 配置文件示例

```yaml
logger:
  default:
  - writer: console
    level: debug
  - writer: file
    level: debug
    filename: ./logs/default.log
    maxsize: 10
    maxage: 15
    maxbackups: 5
    localtime: true
    compress: true

global:
  # 可选：手动指定可分配资源
  # allocatable:
  #   gpu_memory: 16G

apps:
- name: llm-qwen3
  command:
    workdir: ./
    command: "sleep"
    args:
    - "10"
    envs:
    - key: APP_NAME
      value: sleep_a
  restart:
    max_retries: 3
    interval: 5
  replica:
    # 静态副本数，设置为0表示自动调度
    static: 0
    # 需要的GPU内存
    require:
      gpu_memory: 4G
    # 最大副本数
    max_replicas: 2
    # 最小副本数
    min_replicas: 1
```

## API 接口

- GET /ping - 健康检查
- GET /status - 获取状态信息