# Fuck-GPU

在GPU卡上按需运行多个实例，让显存尽可能用满，

## 功能逻辑

1. **资源探测**：通过nvidia-smi获取GPU显存信息
2. **需求分析**：根据应用配置确定每个实例所需的GPU内存
3. **智能调度**：基于可用资源和应用需求，动态确定最佳实例数量
4. **实例管理**：创建、启动、监控和自动重启应用实例
5. **资源优化**：最大化利用GPU显存，避免浪费

## 模板支持

支持在命令和参数中使用模板变量：
- `{{index}}`：实例索引号（从0开始）

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
    level: info
  - writer: file
    level: debug
    filename: ./logs/default.log
    maxsize: 10
    maxage: 15
    maxbackups: 5
    localtime: true
    compress: true

gateway:
  enable: true
  listen_addr: ":8080"

global:
  allocatable:
    gpu_memory: 16G

apps:
- name: llm-qwen3
  command:
    workdir: ./
    command: "python3"
    args:
    - "-m"
    - "http.server"
    - "809{{index}}"
    envs:
    - key: APP_NAME
      value: qwen3_{{index}}
    - key: PORT
      value: "809{{index}}"
  restart:
    # max_retries: 3
    interval: 5s
  replica:
    require:
      gpu_memory: 4G
    max_replicas: 2
    min_replicas: 1
  gateway_backends:
    - path_prefix: "/qwen3"
      backend: "127.0.0.1:809{{index}}"
      health_check:
        path: "/health"
        interval: 2s
        timeout: 1s
        healthy_threshold: 1
        unhealthy_threshold: 1

# - name: echo-app
#   command:
#     workdir: ./
#     command: "echo"
#     args:
#     - "Instance {{index}} started on port 909{{index}}"
#     envs: []
#   restart:
#     max_retries: 0
#   replica:
#     static: 3
#     require:
#       gpu_memory: 0

```

## API 接口

- GET /ping - 健康检查
- GET /status - 获取状态信息
