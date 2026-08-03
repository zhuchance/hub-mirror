---
name: hub-mirror issue template
about: 用于提交镜像同步任务的 Issue 模板
title: "[hub-mirror] 请求同步镜像"
labels: ["hub-mirror"]
---

请在此填写需要同步的镜像地址，每行一个。

示例：

gcr.io/google-samples/microservices-demo/emailservice:v0.3.5
quay.io/jetstack/cert-manager-controller:v1.13.3
ghcr.io/nginx/nginx:latest
nginx:latest

> 注意：
> - 请勿修改 labels（hub-mirror 标签用于触发 workflow）
> - 每行填写一个完整的镜像地址（含 tag）
> - 不要在同一行中填写多个镜像
> - 标题随意，保持阵型即可
