# hub-mirror

使用阿里云容器镜像服务（ACR）提供 gcr.io、k8s.gcr.io、quay.io、ghcr.io 等国外镜像的加速同步与下载服务。

基于 GitHub Actions 实现，纯 bash 驱动，无需任何本地环境。

# 使用

## 1. 直接提交 Issue（使用本项目）

点个 Star ，通过 [提交 Issue](https://github.com/zhuchance/hub-mirror/issues/new/choose) 填写需要同步的镜像地址即可触发自动同步。

要求：每行填写一个完整的镜像地址（含 tag），参考 [Issue 模板](https://github.com/zhuchance/hub-mirror/blob/main/.github/ISSUE_TEMPLATE/hub-mirror.md)

同步完成后，Issue 会自动收到评论，包含可直接执行的 `docker pull` + `docker tag` 脚本，同时在 Actions 运行页面的 Summary 可查看 Hash 校验报告。

## 2. 自己动手，Fork 本项目

### 2.1 开启 Issues 功能

进入仓库 `Settings` - `Options` - `Features`，勾选 `Issues`。

### 2.2 配置 Secrets

进入 `Settings` - `Secrets and variables` - `Actions`，新建以下 Secrets：

| Secret 名称 | 说明 |
| :--- | :--- |
| `ALiyun_USER` | 阿里云容器镜像服务用户名 |
| `ALiyun_PWD` | 阿里云容器镜像服务访问密码 |

### 2.3 添加 Labels

进入 `Issues` - `Labels`，添加三个 label：`hub-mirror`、`success`、`failure`

### 2.4 修改目标仓库地址

编辑 [.github/workflows/sync.yml](https://github.com/zhuchance/hub-mirror/blob/main/.github/workflows/sync.yml) 中的以下配置：

- `registry`：将 `registry.cn-hangzhou.aliyuncs.com` 改为你的阿里云 ACR 地址
- `namespace`：将 `mgfw` 改为你的命名空间
- `secrets`：将 `ALiyun_USER` / `ALiyun_PWD` 改为你自定义的 Secret 名称

### 2.5 启用 Workflow

进入 `Actions` 页面，在左侧选择 `Sync Images & Verify Hash`，点击 `Enable workflow`。

## 3. 在 Actions 页面手动触发

除了通过 Issue 触发外，也可以在 `Actions` 页面点击 `Run workflow`，手动输入镜像列表（每行一个）来触发同步。

# 工作原理

## 同步流程

```
Issue 提交镜像列表
      │
      ▼
GitHub Actions (ubuntu-latest)
      │
      ├─ 1. docker pull 源镜像
      ├─ 2. docker inspect 获取源镜像 Hash (Image ID)
      ├─ 3. docker tag 重命名为阿里云 ACR 地址
      ├─ 4. docker push 推送到阿里云 ACR
      ├─ 5. docker inspect 获取目标镜像 Hash
      └─ 6. 比对源/目标 Hash，生成校验报告
      │
      ▼
Issue 评论返回 pull/tag 脚本 + Summary 报告
```

## 镜像命名规则

源镜像取最后一段作为目标名称，例如：

| 源镜像 | 目标镜像 |
| :--- | :--- |
| `gcr.io/google-samples/microservices-demo/emailservice:v0.3.5` | `registry.cn-hangzhou.aliyuncs.com/mgfw/emailservice:v0.3.5` |
| `quay.io/jetstack/cert-manager-controller:v1.13.3` | `registry.cn-hangzhou.aliyuncs.com/mgfw/cert-manager-controller:v1.13.3` |
| `nginx:latest` | `registry.cn-hangzhou.aliyuncs.com/mgfw/nginx:latest` |

# 教程

## Docker Registry 公开服务

目前常用的 Docker Registry 公开服务有：

- `docker.io`：Docker Hub 官方镜像仓库，也是 Docker 默认的仓库
- `gcr.io`、`k8s.gcr.io`：谷歌镜像仓库
- `quay.io`：Red Hat 镜像仓库
- `ghcr.io`：GitHub 镜像仓库

当使用 `docker pull 仓库地址/用户名/仓库名:标签` 时，会前往对应的仓库地址拉取镜像，标签无声明时默认为 `latest`，仓库地址无声明时默认为 `docker.io`。

众所周知的原因，在国内访问这些服务异常的慢，甚至 `gcr.io` 和 `quay.io` 根本无法访问。

## 解决方案：镜像加速器

针对 `Docker Hub`，Docker 官方和国内各大云服务商均提供了 Docker 镜像加速服务。

你只需要简单配置一下（以 Linux 为例）：

```bash
sudo mkdir -p /etc/docker

sudo tee /etc/docker/daemon.json <<-'EOF'
{
  "registry-mirrors": ["https://26uyfxri.mirror.aliyuncs.com"]
}
EOF

sudo systemctl daemon-reload
sudo service docker restart
```

便可以通过访问国内镜像加速器来加速 `Docker Hub` 的镜像下载。

不过这种办法也只能针对 `docker.io`，其它的仓库地址并没有真正实际可用的加速器。

## 解决方案：用魔法打败魔法

既然无法治本，那治治标还是可以的吧。

若我们使用一台魔法机器从 `gcr.io` 或 `quay.io` 等仓库先把我们无法下载的镜像拉取下来，然后重新上传到国内的镜像仓库（如阿里云 ACR），是不是就可以快速下载了。

GitHub Actions 就是个好选择 —— 它提供了带有 Docker 环境的云端机器，我们可以利用提交 `Issue` 来触发镜像同步的 workflow，将国外镜像仓库的镜像同步到阿里云 ACR。

`workflow` 的实现参考 [sync.yml](https://github.com/zhuchance/hub-mirror/blob/main/.github/workflows/sync.yml)

实际的使用效果参考 [issues](https://github.com/zhuchance/hub-mirror/issues?q=is%3Aissue+is%3Aopen+label%3Ahub-mirror)

同步完成后，只需在 Issue 评论中复制 `docker pull` + `docker tag` 脚本执行，就可以飞快地使用国内镜像仓库下载 `gcr.io` 或 `quay.io` 等镜像了。
