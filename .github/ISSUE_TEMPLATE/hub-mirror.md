---
name: hub-mirror issue template
about: 用于执行镜像同步任务的 Issue 模板
title: "[sync] 请求同步镜像"
labels: ["hub-mirror"]
---

请在此填写需要同步的镜像地址，每行一个，最多 20 个。

示例：

gcr.io/google-samples/microservices-demo/emailservice:v0.3.5
quay.io/jetstack/cert-manager-controller:v1.13.3
ghcr.io/nginx/nginx:latest
nginx:latest

> 注意：
> - 标题需包含 `sync` 关键词以触发 workflow
> - 每行填写一个完整的镜像地址（含 tag）
> - 不要在同一行中填写多个镜像
> - 不要修改 labels
